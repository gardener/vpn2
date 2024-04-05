#!/bin/bash -e
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

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
fileVPNNetwork=

[ -e "${baseConfigDir}/serviceNetwork" ] && fileServiceNetwork=$(cat ${baseConfigDir}/serviceNetwork)
[ -e "${baseConfigDir}/podNetwork" ] && filePodNetwork=$(cat ${baseConfigDir}/podNetwork)
[ -e "${baseConfigDir}/nodeNetwork" ] && fileNodeNetwork=$(cat ${baseConfigDir}/nodeNetwork)
[ -e "${baseConfigDir}/vpnNetwork" ] && fileVPNNetwork=$(cat ${baseConfigDir}/vpnNetwork)

service_network="${SERVICE_NETWORK:-${fileServiceNetwork}}"
service_network="${service_network:-100.64.0.0/13}"
pod_network="${POD_NETWORK:-${filePodNetwork}}"
pod_network="${pod_network:-100.96.0.0/11}"
node_network="${NODE_NETWORK:-${fileNodeNetwork}}"
node_network="${node_network:-}"
# defaults for vpn_network are set depending on IP_FAMILIES below
vpn_network="${VPN_NETWORK:-${fileVPNNetwork}}"

is_ha=
if [[ $POD_NAME =~ .*-([0-2])$ ]]; then
  is_ha=true
  vpn_index=${BASH_REMATCH[1]}
fi

first_three_octets_of_ipv4_vpn=

if [[ "$IP_FAMILIES" = "IPv4" ]]; then
  # set IPv4 default if no config has been provided
  vpn_network="${vpn_network:-"192.168.123.0/24"}"

  if [[ $vpn_network != */24 ]]; then
    log "error: the IPv4 VPN setup requires the VPN network range to have a /24 suffix"
    exit 1
  fi

  # it's guaranteed that the VPN network range is a /24 net,
  # so it's safe to just cut off the last octet and net size
  first_three_octets_of_ipv4_vpn=${vpn_network%.*}

  if [[ -n $is_ha ]]; then
    # HA VPN tunnels split the /24 VPN network into four /26 ranges:
    # vpn-server-0: first /26
    # vpn-server-1: second /26
    # vpn-server-2: third /26 (optional)
    # bonding:      fourth /26
    openvpn_network="${first_three_octets_of_ipv4_vpn}.$((vpn_index * 64))/26"
    pool_start_ip="${first_three_octets_of_ipv4_vpn}.$((vpn_index * 64 + 8))"
    pool_end_ip="${first_three_octets_of_ipv4_vpn}.$((vpn_index * 64 + 62))"
  else
    openvpn_network=${vpn_network}
    pool_start_ip="${first_three_octets_of_ipv4_vpn}.10"
    pool_end_ip="${first_three_octets_of_ipv4_vpn}.254"
  fi
else
  # set IPv6 default if no config has been provided
  vpn_network="${vpn_network:-"fd8f:6d53:b97a:1::/120"}"

  if [[ $vpn_network != */120 ]]; then
    log "error: the IPv6 VPN setup requires the VPN network range to have a /120 suffix"
    exit 1
  fi

  if [[ -n $is_ha ]]; then
    log "error: the highly-available VPN setup is only supported for IPv4 single-stack shoots"
    exit 1
  fi

  openvpn_network=${vpn_network}
fi

log "using openvpn_network=$openvpn_network"

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

data-ciphers AES-256-GCM:AES-256-CBC

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
dh none

auth SHA256
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
    printf 'ifconfig-push %s %s\n' "${first_three_octets_of_ipv4_vpn}.$((vpn_index * 64 + i + 2))" "$(CIDR2Netmask $openvpn_network)" >/client-config-dir/vpn-shoot-client-$i
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
