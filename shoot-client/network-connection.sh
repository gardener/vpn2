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

if [[ -z "$DO_NOT_CONFIGURE_KERNEL_SETTINGS" ]]; then
  configure_tcp

  # make sure forwarding is enabled
  echo 1 > /proc/sys/net/ipv4/ip_forward
fi

if [[ ! -z "$EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS" ]]; then
  exit
fi

# for each cidr config, it looks first at its env var, then a local file (which may be a volume mount), then the default
baseConfigDir="/init-config"
fileServiceNetwork=
filePodNetwork=
fileNodeNetwork=
[ -e "${baseConfigDir}/serviceNetwork" ] && fileServiceNetwork=$(cat ${baseConfigDir}/serviceNetwork)
[ -e "${baseConfigDir}/podNetwork" ] && filePodNetwork=$(cat ${baseConfigDir}/podNetwork)
[ -e "${baseConfigDir}/nodeNetwork" ] && fileNodeNetwork=$(cat ${baseConfigDir}/nodeNetwork)

service_network="${SERVICE_NETWORK:-${fileServiceNetwork}}"
service_network="${service_network:-100.64.0.0/13}"
pod_network="${POD_NETWORK:-${filePodNetwork}}"
pod_network="${pod_network:-100.96.0.0/11}"
node_network="${NODE_NETWORK:-${fileNodeNetwork}}"
node_network="${node_network:-}"

reversed_vpn_header="${REVERSED_VPN_HEADER:-invalid-host}"

# calculate netmask for given CIDR (required by openvpn)
CIDR2Netmask() {
    local cidr="$1"

    local ip=$(echo $cidr | cut -f1 -d/)
    local numon=$(echo $cidr | cut -f2 -d/)

    local numoff=$(( 32 - $numon ))
    while [ "$numon" -ne "0" ]; do
            start=1${start}
            numon=$(( $numon - 1 ))
    done
    while [ "$numoff" -ne "0" ]; do
        end=0${end}
        numoff=$(( $numoff - 1 ))
    done
    local bitstring=$start$end

    bitmask=$(echo "obase=16 ; $(( 2#$bitstring )) " | bc | sed 's/.\{2\}/& /g')

    for t in $bitmask ; do
        str=$str.$((16#$t))
    done

    echo $str | cut -f2-  -d\.
}

service_network_address=$(echo $service_network | cut -f1 -d/)
service_network_netmask=$(CIDR2Netmask $service_network)

pod_network_address=$(echo $pod_network | cut -f1 -d/)
pod_network_netmask=$(CIDR2Netmask $pod_network)

sed -e "s/\${SERVICE_NETWORK_ADDRESS}/${service_network_address}/" \
    -e "s/\${SERVICE_NETWORK_NETMASK}/${service_network_netmask}/" \
    -e "s/\${POD_NETWORK_ADDRESS}/${pod_network_address}/" \
    -e "s/\${POD_NETWORK_NETMASK}/${pod_network_netmask}/" \
    openvpn.config.template > openvpn.config

if [[ ! -z "$node_network" ]]; then
  for n in $(echo $node_network |  sed 's/[][]//g' | sed 's/,/ /g')
  do
      node_network_address=$(echo $n | cut -f1 -d/)
      node_network_netmask=$(CIDR2Netmask $n)
      echo "pull-filter ignore \"route ${node_network_address} ${node_network_netmask}\"" >> openvpn.config
  done
fi

echo "pull-filter accept \"route 192.168.123.\"" >> openvpn.config
echo "pull-filter ignore \"route\"" >> openvpn.config
echo "pull-filter ignore redirect-gateway" >> openvpn.config
echo "pull-filter ignore route-ipv6" >> openvpn.config
echo "pull-filter ignore redirect-gateway-ipv6" >> openvpn.config

# enable forwarding and NAT
iptables --append FORWARD --in-interface tun0 -j ACCEPT
iptables --append POSTROUTING --out-interface eth0 --table nat -j MASQUERADE

if [[ ! -c /dev/net/tun ]]; then
    mkdir -p /dev/net
    mknod /dev/net/tun c 10 200
fi

while : ; do
    if [[ ! -z $ENDPOINT ]]; then
        openvpn --remote ${ENDPOINT} --port ${openvpn_port} --http-proxy ${ENDPOINT} ${openvpn_port} --http-proxy-option CUSTOM-HEADER Reversed-VPN "${reversed_vpn_header}" --config openvpn.config
    else
        log "No tunnel endpoint found"
    fi
    sleep 1
done
