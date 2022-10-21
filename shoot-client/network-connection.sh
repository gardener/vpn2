#!/bin/bash -e
#
# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function log() {
  echo "[$(date -u)]: $*"
}

trap 'exit' TERM SIGINT

openvpn_port="${OPENVPN_PORT:-8132}"

tcp_keepalive_time="${TCP_KEEPALIVE_TIME:-7200}"
tcp_keepalive_intvl="${TCP_KEEPALIVE_INTVL:-75}"
tcp_keepalive_probes="${TCP_KEEPALIVE_PROBES:-9}"
tcp_retries2="${TCP_RETRIES2:-5}"

ENDPOINT="${ENDPOINT}"

function set_value() {
  if [ -f $1 ] ; then
    log "Setting $2 on $1"
    echo "$2" > $1
  fi
}

function configure_tcp() {
  set_value /proc/sys/net/ipv4/tcp_keepalive_time $tcp_keepalive_time
  set_value /proc/sys/net/ipv4/tcp_keepalive_intvl $tcp_keepalive_intvl
  set_value /proc/sys/net/ipv4/tcp_keepalive_probes $tcp_keepalive_probes

  set_value /proc/sys/net/ipv4/tcp_retries2 $tcp_retries2
}

function add_iptables_rule() {
  rule=$1

  set +e
  iptables -C $rule > /dev/null
  rc=$?
  set -e
  if [[ "$rc" != "0" ]]; then
    iptables -A $rule
  fi
}

if [[ "$DO_NOT_CONFIGURE_KERNEL_SETTINGS" != "true" ]]; then
  log "configure kernel settings"
  configure_tcp

  # make sure forwarding is enabled
  echo 1 > /proc/sys/net/ipv4/ip_forward
fi

if [[ -n "$EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS" ]]; then
  exit
fi

if [[ -n "$REVERSED_VPN_HEADER" ]]; then
  is_shoot_client="true"
fi

reversed_vpn_header="${REVERSED_VPN_HEADER:-invalid-host}"

vpn_seed_server="vpn-seed-server"
dev="tun0"
if [[ -n "$VPN_SERVER_INDEX" ]]; then
  vpn_seed_server="vpn-seed-server-$VPN_SERVER_INDEX"
  dev="tap$VPN_SERVER_INDEX"
fi
log "using $vpn_seed_server, dev $dev"

# suffix for vpn client secret directory
suffix=""
if [[ $POD_NAME =~ .*-([0-4])$ ]]; then
  suffix="-${BASH_REMATCH[1]}"
fi

sed -e "s/\${SUFFIX}/${suffix}/" \
    openvpn.config.template > openvpn.config

echo "pull-filter ignore redirect-gateway" >> openvpn.config
echo "pull-filter ignore route-ipv6" >> openvpn.config
echo "pull-filter ignore redirect-gateway-ipv6" >> openvpn.config

echo "port ${openvpn_port}" >> openvpn.config
if [[ "$is_shoot_client" == "true" ]]; then
  # use http proxy only for vpn-shoot-client
  echo "http-proxy ${ENDPOINT} ${openvpn_port}" >> openvpn.config
  echo "http-proxy-option CUSTOM-HEADER Reversed-VPN ${reversed_vpn_header}" >> openvpn.config

  # enable forwarding and NAT
  iptables --append FORWARD --in-interface $dev -j ACCEPT
  iptables --append POSTROUTING --out-interface eth0 --table nat -j MASQUERADE
else
  if [[ "$VPN_SERVER_INDEX" == "0" ]]; then
    # start vpn-path-controller for selecting routing path
    log "starting vpn-path-controller (logs see /path-controller.log)"
    /path-controller.sh &
  fi

  # Add firewall rules to block all traffic originating from the shoot cluster.
  add_iptables_rule "INPUT -m state --state RELATED,ESTABLISHED -i $dev -j ACCEPT"
  add_iptables_rule "INPUT -i $dev -j DROP"
fi

while : ; do
  if [[ -n $ENDPOINT ]]; then
    log "openvpn --dev $dev --remote $ENDPOINT --config openvpn.config"
    openvpn --dev $dev --remote $ENDPOINT --config openvpn.config
  else
    log "No tunnel endpoint found"
  fi
  sleep 1
done
