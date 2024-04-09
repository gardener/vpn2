#!/bin/bash -e
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

trap 'exit' TERM SIGINT

loglen=0
function log() {
    echo "[$(date -u)]: $*"
}

# apply env var defaults
IP_FAMILIES="${IP_FAMILIES:-IPv4}"

bondPrefix=
bondBits="26"
bondStart="192"

if [[ $IP_FAMILIES == "IPv4" ]]; then
  # set IPv4 default if no config has been provided
  vpn_network="${VPN_NETWORK:-"192.168.123.0/24"}"

  if [[ $vpn_network != */24 ]]; then
    log "error: the IPv4 VPN setup requires the VPN network range to have a /24 suffix"
    exit 1
  fi

  # it's guaranteed that the VPN network range is a /24 net,
  # so it's safe to just cut off the last octet and net size
  first_three_octets_of_ipv4_vpn=${vpn_network%.*}

  # cidr for bonding network: last /26 subnet of the /24 VPN network range
  bondPrefix=${first_three_octets_of_ipv4_vpn}
else
  # set IPv6 default if no config has been provided
  vpn_network="${VPN_NETWORK:-"fd8f:6d53:b97a:1::/120"}"

  if [[ $vpn_network != */120 ]]; then
    log "error: the IPv6 VPN setup requires the VPN network range to have a /120 suffix"
    exit 1
  fi

  # the highly-available VPN setup is only supported for IPv4 single-stack shoots
  # hence, the bonding-related calculations are not performed here
fi

for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
  ip="${bondPrefix}.$((bondStart+c+2))"
  logline+="$((bondStart+c+2))=\${ping_return_msg[$ip]} "
done
logline+=' using $new_ip'
new_ip=""

if [[ -n "${NODE_NETWORK}" ]]; then
  check_network="${NODE_NETWORK}"
else
  check_network="${SERVICE_NETWORK}"
fi

declare -A ping_pid
declare -A ping_return
declare -A ping_return_msg

function pingAllShootClients() {
    set +e
    for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
        ip="${bondPrefix}.$((bondStart+c+2))"
        ping -W 2 -w 2 -c 1 $ip > /dev/null &
        ping_pid[$ip]=$!
    done

    local result=$(ip route list ${check_network})
    old_ip=
    if [[ $result =~ via[[:blank:]]([0-9.]+)[[:blank:]] ]]; then
      old_ip="${BASH_REMATCH[1]}"
    fi

    for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
        ip="${bondPrefix}.$((bondStart+c+2))"
        wait ${ping_pid[$ip]}
        ping_return[$ip]=$?
        if [[ "${ping_return[$ip]}" == "0" ]]; then
          ping_return_msg[$ip]="ok"
        else
          ping_return_msg[$ip]="err"
        fi
    done
    set -e
}

function selectNewShootClient() {
    local good=()
    for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
        ip="${bondPrefix}.$((bondStart+c+2))"
        if [[ "${ping_return[$ip]}" == "0" ]]; then
            good+=($ip)
        fi
    done
    local len=${#good[@]}
    if (( len > 0 )); then
        # select random good path
        new_ip=${good[$(( $RANDOM % len ))]}
    else
        # keep last value
        new_ip=$old_ip
    fi
}

function updateRouting() {
    log "switching from $old_ip to $new_ip"

    # ensure routes
    ip route replace ${POD_NETWORK} dev bond0 via $new_ip
    ip route replace ${SERVICE_NETWORK} dev bond0 via $new_ip
    if [[ -n "${NODE_NETWORK}" ]]; then
      ip route replace ${NODE_NETWORK} dev bond0 via $new_ip
    fi
    old_ip=$new_ip
}

while : ; do
    pingAllShootClients

    new_ip=$old_ip
    if [[ "$old_ip" == "" || "${ping_return[$old_ip]}" != "0" ]]; then
        selectNewShootClient
    fi

    log $(eval echo $logline)
    if [[ "$old_ip" != "$new_ip" ]]; then
        updateRouting
    fi
    sleep 2
done
