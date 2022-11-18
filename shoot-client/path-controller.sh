#!/bin/bash -e
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

trap 'exit' TERM SIGINT

loglen=0
function log() {
    echo "[$(date -u)]: $*"
}

bondPrefix="192.168.122"

for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
  ip="${bondPrefix}.$((c+10))"
  logline+="$((c+10))=\${ping_return[$ip]} "
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

function pingAllShootClients() {
    set +e
    for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
        ip="${bondPrefix}.$((c+10))"
        ping -W 2 -w 2 -c 1 $ip > /dev/null &
        ping_pid[$ip]=$!
    done

    local result=$(ip route list ${check_network})
    old_ip=
    if [[ $result =~ via[[:blank:]]([0-9.]+)[[:blank:]] ]]; then
      old_ip="${BASH_REMATCH[1]}"
    fi

    for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
        ip="${bondPrefix}.$((c+10))"
        wait ${ping_pid[$ip]}
        ping_return[$ip]=$?
    done
    set -e
}

function selectNewShootClient() {
    local good=()
    for (( c=0; c<$HA_VPN_CLIENTS; c++ )); do
        ip="${bondPrefix}.$((c+10))"
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
