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

# for each cidr config, it looks first at its env var, then a local file (which may be a volume mount), then the default
baseConfigDir="/init-config"
fileServiceNetwork=
filePodNetwork=
fileNodeNetwork=
[ -e "${baseConfigDir}/serviceNetwork" ] && fileServiceNetwork=$(cat ${baseConfigDir}/serviceNetwork)
[ -e "${baseConfigDir}/podNetwork" ] && filePodNetwork=$(cat ${baseConfigDir}/podNetwork)
[ -e "${baseConfigDir}/nodeNetwork" ] && fileNodeNetwork=$(cat ${baseConfigDir}/nodeNetwork)

is_ha=
if [[ $POD_NAME =~ .*-([0-2])$ ]]; then
  is_ha=true
  vpn_index=${BASH_REMATCH[1]}
fi

if [[ -n $is_ha ]]; then
  if [[ "$IP_FAMILIES" != "IPv4" ]]; then
    log "error: the highly-available VPN setup is only supported for IPv4 single-stack shoots"
    exit 1
  fi

  # HA VPN tunnels split the 192.168.123.0/24 into four ranges:
  # vpn-server-0: 192.168.123.0/26
  # vpn-server-1: 192.168.123.64/26
  # vpn-server-2: 192.168.123.128/26 (optional)
  # bonding:      192.168.123.192/26
  openvpn_network="192.168.123.$((vpn_index * 64))/26"
  pool_start_ip="192.168.123.$((vpn_index * 64 + 8))"
  pool_end_ip="192.168.123.$((vpn_index * 64 + 62))"
else
  if [[ "$IP_FAMILIES" = "IPv4" ]]; then
    openvpn_network="192.168.123.0/24"
    pool_start_ip="192.168.123.10"
    pool_end_ip="192.168.123.254"
  else
    openvpn_network="fd8f:6d53:b97a:1::/120"
  fi
fi

log "using openvpn_network=$openvpn_network"

service_network="${SERVICE_NETWORK:-${fileServiceNetwork}}"
service_network="${service_network:-100.64.0.0/13}"
pod_network="${POD_NETWORK:-${filePodNetwork}}"
pod_network="${pod_network:-100.96.0.0/11}"
node_network="${NODE_NETWORK:-${fileNodeNetwork}}"
node_network="${node_network:-}"

# calculate netmask for given CIDR (required by openvpn)
#
CIDR2Netmask() {
  local cidr="$1"

  local ip=$(echo $cidr | cut -f1 -d/)
  local numon=$(echo $cidr | cut -f2 -d/)

  local numoff=$((32 - $numon))
  local start
  local end
  while [ "$numon" -ne "0" ]; do
    start=1${start}
    numon=$(($numon - 1))
  done
  while [ "$numoff" -ne "0" ]; do
    end=0${end}
    numoff=$(($numoff - 1))
  done
  local bitstring=$start$end

  local bitmask=$(echo "obase=16 ; $((2#$bitstring)) " | bc | sed 's/.\{2\}/& /g')

  local str
  for t in $bitmask; do
    str=$str.$((16#$t))
  done

  echo $str | cut -f2- -d\.
}

# Write default config
cat >openvpn.config <<EOF
mode server
tls-server
topology subnet

# Additonal optimizations
txqueuelen 1000

cipher AES-256-CBC
data-ciphers AES-256-CBC

# port can always be 1194 here as it is not visible externally. A different
# port can be configured for the external load balancer in the service
# manifest
port 1194

keepalive 10 60

# client-config-dir to push client specific configuration
client-config-dir /client-config-dir

key "/srv/secrets/vpn-server/tls.key"
cert "/srv/secrets/vpn-server/tls.crt"
ca "/srv/secrets/vpn-server/ca.crt"
dh "/srv/secrets/dh/dh2048.pem"

tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
EOF

# Write config that is dependent on the IP family
if [[ "$IP_FAMILIES" = "IPv4" ]]; then
  {
    printf 'proto tcp4-server\n'
    printf 'server %s %s nopool\n' "$(echo $openvpn_network | cut -f1 -d/)" "$(CIDR2Netmask $openvpn_network)"
    printf 'ifconfig-pool %s %s\n' "$pool_start_ip" "$pool_end_ip"
  } >>openvpn.config

  {
    printf 'iroute %s %s\n' "$(echo $service_network | cut -f1 -d/)" "$(CIDR2Netmask $service_network)"
    printf 'iroute %s %s\n' "$(echo $pod_network | cut -f1 -d/)" "$(CIDR2Netmask $pod_network)"
  } >/client-config-dir/vpn-shoot-client
else
  {
    printf 'proto tcp6-server\n'
    printf 'server-ipv6 %s\n' "$openvpn_network"
  } >>openvpn.config

  {
    printf 'iroute-ipv6 %s\n' "$service_network"
    printf 'iroute-ipv6 %s\n' "$pod_network"
  } >/client-config-dir/vpn-shoot-client
fi

if [[ -n $is_ha ]]; then
  dev="tap0"
  echo "client-to-client" >>openvpn.config
  echo "duplicate-cn" >>openvpn.config

  for ((i = 0; i < $HA_VPN_CLIENTS; i++)); do
    printf 'ifconfig-push %s %s\n' "192.168.123.$((vpn_index * 64 + i + 2))" "$(CIDR2Netmask $openvpn_network)" >/client-config-dir/vpn-shoot-client-$i
  done
else
  dev="tun0"

  if [[ "$IP_FAMILIES" = "IPv4" ]]; then
    {
      printf "route %s %s\n" "$(echo $service_network | cut -f1 -d/)" "$(CIDR2Netmask $service_network)"
      printf "route %s %s\n" "$(echo $pod_network | cut -f1 -d/)" "$(CIDR2Netmask $pod_network)"
    } >>openvpn.config
  else
    {
      printf "route-ipv6 %s\n" "$service_network"
      printf "route-ipv6 %s\n" "$pod_network"
    } >>openvpn.config
  fi

  if [[ -n "$node_network" ]]; then
    for n in $(echo $node_network | sed 's/[][]//g' | sed 's/,/ /g'); do
      if [[ "$IP_FAMILIES" = "IPv4" ]]; then
        node_network_address=$(echo $n | cut -f1 -d/)
        node_network_netmask=$(CIDR2Netmask $n)
        printf 'route %s %s\n' "${node_network_address}" "${node_network_netmask}" >>openvpn.config
        printf 'iroute %s %s\n' "${node_network_address}" "${node_network_netmask}" >>/client-config-dir/vpn-shoot-client
      else
        node_network_address="$n"
        printf 'route-ipv6 %s\n' "${node_network_address}" >>openvpn.config
        printf 'iroute-ipv6 %s\n' "${node_network_address}" >>/client-config-dir/vpn-shoot-client
      fi
    done
  fi
fi

echo "dev $dev" >>openvpn.config

# Add firewall rules to block all traffic originating from the shoot cluster.
# The scripts are run after the tun device has been created (up) or removed
# (down).
echo "script-security 2" >>openvpn.config
echo "up \"/firewall.sh on $dev\"" >>openvpn.config
echo "down \"/firewall.sh off $dev\"" >>openvpn.config

if [[ -n "$OPENVPN_STATUS_PATH" ]]; then
  echo "status \"$OPENVPN_STATUS_PATH\" 15" >>openvpn.config
  echo "status-version 2" >>openvpn.config
fi

local_node_ip="${LOCAL_NODE_IP:-255.255.255.255}"

# filter log output to remove readiness/liveness probes from local node
openvpn --config openvpn.config | grep -v -E "(TCP connection established with \[AF_INET(6)?\]${local_node_ip}|)?${local_node_ip}(:[0-9]{1,5})? Connection reset, restarting"
