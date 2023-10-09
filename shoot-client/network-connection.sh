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

# apply env var defaults
IP_FAMILIES="${IP_FAMILIES:-IPv4}"
openvpn_port="${OPENVPN_PORT:-8132}"

iptables=iptables
if [[ "$IP_FAMILIES" = "IPv6" ]]; then
  iptables=ip6tables
fi

# cidr for bonding network: 192.168.123.192/26
bondPrefix="192.168.123"
bondBits="26"
bondStart="192"

tcp_keepalive_time="${TCP_KEEPALIVE_TIME:-7200}"
tcp_keepalive_intvl="${TCP_KEEPALIVE_INTVL:-75}"
tcp_keepalive_probes="${TCP_KEEPALIVE_PROBES:-9}"
tcp_retries2="${TCP_RETRIES2:-5}"

ENDPOINT="${ENDPOINT}"

function set_value() {
  if [ -f $1 ]; then
    log "Setting $2 on $1"
    echo "$2" >$1
  fi
}

function configure_tcp() {
  set_value /proc/sys/net/ipv4/tcp_keepalive_time $tcp_keepalive_time
  set_value /proc/sys/net/ipv4/tcp_keepalive_intvl $tcp_keepalive_intvl
  set_value /proc/sys/net/ipv4/tcp_keepalive_probes $tcp_keepalive_probes

  set_value /proc/sys/net/ipv4/tcp_retries2 $tcp_retries2
}

function configure_bonding() {
  local addr
  local targets

  if [[ "$IS_SHOOT_CLIENT" == "true" ]]; then
    # IP address is fixed on shoot side
    addr="${bondPrefix}.$((bondStart + vpn_client_index + 2))/$bondBits"
    targets="${bondPrefix}.$((bondStart + 1))" # using a dummy address as kube-apiserver IPs are unknown
  else
    # for each kube-apiserver pod acquire an IP via consensus
    # based on pod annotations (details see go part)
    log "acquiring ip address for bonding"
    OUTPUT=/tmp/acquired-ip ./acquire-ip
    addr="$(</tmp/acquired-ip)/$bondBits"

    for ((i = 0; i < $HA_VPN_CLIENTS; i++)); do
      if ((i > 0)); then
        targets+=','
      fi
      targets+="${bondPrefix}.$((bondStart + i + 2))"
    done
  fi
  log "bonding address is $addr"

  ip link del bond0 2>/dev/null || true
  for ((i = 0; i < $HA_VPN_SERVERS; i++)); do
    # create tunnel devices
    ip link del tap$i 2>/dev/null || true
    openvpn --mktun --dev tap$i
  done
  # use bonding
  # - with active-backup mode
  # - activate ARP requests (but not used for monitoring as use_carrier=1 and arp_validate=none by default)
  # - using `primary tap0` to avoid ambiguity of selection if multiple devices are up (primary_reselect=always by default)
  # - using `num_grat_arp 5` as safe-guard on switching device
  local cmd=$(echo ip link add bond0 type bond mode active-backup fail_over_mac 1 arp_interval 1000 arp_ip_target \"$targets\" arp_all_targets 0 primary tap0 num_grat_arp 5)
  log $cmd
  $(eval echo $cmd)
  for ((i = 0; i < $HA_VPN_SERVERS; i++)); do
    # make tunnel devices slaves of bond0
    ip link set tap$i master bond0
  done
  ip link set bond0 up
  ip addr add $addr dev bond0
}

function add_iptables_rule() {
  rule=$1

  set +e
  $iptables -C $rule >/dev/null
  rc=$?
  set -e
  if [[ "$rc" != "0" ]]; then
    $iptables -A $rule
  fi
}

if [[ "$DO_NOT_CONFIGURE_KERNEL_SETTINGS" != "true" ]]; then
  log "configure kernel settings"
  configure_tcp

  # make sure forwarding is enabled
  echo 1 >/proc/sys/net/ipv4/ip_forward
  echo 1 >/proc/sys/net/ipv6/conf/all/forwarding
fi

# suffix for vpn client secret directory
suffix=""
if [[ "$IS_SHOOT_CLIENT" == "true" ]]; then
  if [[ $POD_NAME =~ .*-([0-2])$ ]]; then
    suffix="-${BASH_REMATCH[1]}"
    vpn_client_index="${BASH_REMATCH[1]}"
  fi
fi

if [[ "$CONFIGURE_BONDING" == "true" ]]; then
  # HA VPN is currently not supported in combination with IPv6
  log "configure bonding"
  configure_bonding
fi

if [[ -n "$EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS" ]]; then
  exit
fi

reversed_vpn_header="${REVERSED_VPN_HEADER:-invalid-host}"

vpn_seed_server="vpn-seed-server"
dev="tun0"
forward_device="tun0"
if [[ -n "$VPN_SERVER_INDEX" ]]; then
  vpn_seed_server="vpn-seed-server-$VPN_SERVER_INDEX"
  dev="tap$VPN_SERVER_INDEX"
  forward_device="bond0"
fi
log "using $vpn_seed_server, dev $dev"

# Write default config
cat >openvpn.config <<EOF
# use TCP instead of UDP (commonly not supported by load balancers)

# don't cache authorization information in memory
auth-nocache

# stop process if something goes wrong
remap-usr1 SIGTERM

# Additonal optimizations
txqueuelen 1000

# get all routing information from server
pull

cipher AES-256-CBC
data-ciphers AES-256-CBC

tls-client

tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 1

# https://openvpn.net/index.php/open-source/documentation/howto.html#mitm
remote-cert-tls server
EOF

# Write config that is dependent on the IP family
if [[ "$IP_FAMILIES" = "IPv4" ]]; then
  printf 'proto tcp4-client\n' >>openvpn.config
else
  printf 'proto tcp6-client\n' >>openvpn.config
fi

{
  printf 'key /srv/secrets/vpn-client%s/tls.key\n' "$suffix"
  printf 'cert /srv/secrets/vpn-client%s/tls.crt\n' "$suffix"
  printf 'ca /srv/secrets/vpn-client%s/ca.crt\n' "$suffix"
} >>openvpn.config

echo "pull-filter ignore redirect-gateway" >>openvpn.config
echo "pull-filter ignore  redirect-gateway-ipv6" >>openvpn.config

echo "port ${openvpn_port}" >>openvpn.config
if [[ "$IS_SHOOT_CLIENT" == "true" ]]; then
  # use http proxy only for vpn-shoot-client
  echo "http-proxy ${ENDPOINT} ${openvpn_port}" >>openvpn.config
  echo "http-proxy-option CUSTOM-HEADER Reversed-VPN ${reversed_vpn_header}" >>openvpn.config

  # enable forwarding and NAT
  if [[ "$IP_FAMILIES" = "IPv4" ]]; then
    $iptables --append FORWARD --in-interface $forward_device -j ACCEPT
  fi
  $iptables --append POSTROUTING --out-interface eth0 --table nat -j MASQUERADE
else
  # Add firewall rules to block all traffic originating from the shoot cluster.
  add_iptables_rule "INPUT -m state --state RELATED,ESTABLISHED -i $forward_device -j ACCEPT"
  add_iptables_rule "INPUT -i $forward_device -j DROP"
fi

while :; do
  if [[ -n $ENDPOINT ]]; then
    log "openvpn --dev $dev --remote $ENDPOINT --config openvpn.config"
    openvpn --dev $dev --remote $ENDPOINT --config openvpn.config
  else
    log "No tunnel endpoint found"
  fi
  sleep 1
done
