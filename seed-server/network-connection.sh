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

# for each cidr config, it looks first at its env var, then a local file (which may be a volume mount), then the default
baseConfigDir="/init-config"
fileServiceNetwork=
filePodNetwork=
fileNodeNetwork=
[ -e "${baseConfigDir}/serviceNetwork" ] && fileServiceNetwork=$(cat ${baseConfigDir}/serviceNetwork)
[ -e "${baseConfigDir}/podNetwork" ] && filePodNetwork=$(cat ${baseConfigDir}/podNetwork)
[ -e "${baseConfigDir}/nodeNetwork" ] && fileNodeNetwork=$(cat ${baseConfigDir}/nodeNetwork)

n=123
is_ha=
if [[ $POD_NAME =~ .*-([0-4])$ ]]; then
  is_ha=true
  n=$((123 + ${BASH_REMATCH[1]}))
fi
openvpn_prefix="192.168.${n}"
openvpn_network="${openvpn_prefix}.0/24"
pool_start_ip="${openvpn_prefix}.32"
pool_end_ip="${openvpn_prefix}.254"
echo "using openvpn_network=$openvpn_network"

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


openvpn_network_address=$(echo $openvpn_network | cut -f1 -d/)
openvpn_network_netmask=$(CIDR2Netmask $openvpn_network)

service_network_address=$(echo $service_network | cut -f1 -d/)
service_network_netmask=$(CIDR2Netmask $service_network)

pod_network_address=$(echo $pod_network | cut -f1 -d/)
pod_network_netmask=$(CIDR2Netmask $pod_network)

sed -e "s/\${SERVICE_NETWORK_ADDRESS}/${service_network_address}/" \
    -e "s/\${SERVICE_NETWORK_NETMASK}/${service_network_netmask}/" \
    -e "s/\${POD_NETWORK_ADDRESS}/${pod_network_address}/" \
    -e "s/\${POD_NETWORK_NETMASK}/${pod_network_netmask}/" \
    -e "s/\${OPENVPN_NETWORK_ADDRESS}/${openvpn_network_address}/" \
    -e "s/\${OPENVPN_NETWORK_NETMASK}/${openvpn_network_netmask}/" \
    -e "s/\${POOL_START_IP}/${pool_start_ip}/" \
    -e "s/\${POOL_END_IP}/${pool_end_ip}/" \
    openvpn.config.template > openvpn.config

sed -e "s/\${SERVICE_NETWORK_ADDRESS}/${service_network_address}/" \
    -e "s/\${SERVICE_NETWORK_NETMASK}/${service_network_netmask}/" \
    -e "s/\${POD_NETWORK_ADDRESS}/${pod_network_address}/" \
    -e "s/\${POD_NETWORK_NETMASK}/${pod_network_netmask}/" \
    /client-config-dir/vpn-shoot-client.template > /client-config-dir/vpn-shoot-client

if [[ -n $is_ha ]]; then
    for ((i=0; i < 2; i++))
    do
        sed -e "s/\${SERVICE_NETWORK_ADDRESS}/${service_network_address}/" \
            -e "s/\${SERVICE_NETWORK_NETMASK}/${service_network_netmask}/" \
            -e "s/\${POD_NETWORK_ADDRESS}/${pod_network_address}/" \
            -e "s/\${POD_NETWORK_NETMASK}/${pod_network_netmask}/" \
            -e "s/\${LOCAL_ADDRESS}/${openvpn_prefix}.$((i + 2))/" \
            -e "s/\${OPENVPN_NETWORK_NETMASK}/${openvpn_network_netmask}/" \
            /client-config-dir/vpn-shoot-client-n.template > /client-config-dir/vpn-shoot-client-$i
    done
fi

if [[ -n $is_ha ]]; then
    dev="tap0"
    echo "client-to-client" >> openvpn.config
    echo "duplicate-cn" >> openvpn.config
else 
    dev="tun0"
    echo "route ${service_network_address} ${service_network_netmask}" >> openvpn.config
    echo "route ${pod_network_address} ${pod_network_netmask}" >> openvpn.config

    if [[ ! -z "$node_network" ]]; then
        for n in $(echo $node_network |  sed 's/[][]//g' | sed 's/,/ /g')
        do
            node_network_address=$(echo $n | cut -f1 -d/)
            node_network_netmask=$(CIDR2Netmask $n)
            echo "route ${node_network_address} ${node_network_netmask}" >> openvpn.config
            echo "iroute ${node_network_address} ${node_network_netmask}" >> /client-config-dir/vpn-shoot-client
        done
    fi
fi

echo "dev $dev" >> openvpn.config

# Add firewall rules to block all traffic originating from the shoot cluster.
# The scripts are run after the tun device has been created (up) or removed
# (down).
echo "script-security 2" >> openvpn.config
echo "up \"/firewall.sh on $dev\"" >> openvpn.config
echo "down \"/firewall.sh off $dev\"" >> openvpn.config


local_node_ip="${LOCAL_NODE_IP:-255.255.255.255}"

# filter log output to remove readiness/liveness probes from local node
openvpn --config openvpn.config | grep -v -E "TCP connection established with \[AF_INET\]${local_node_ip}|${local_node_ip}:[0-9]{1,5} Connection reset, restarting"
