#!/bin/bash
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
    echo "[$(date -u)]: $*" >> /path-controller.log
    ((loglen++))
    if (( $loglen > 1800 )); then
        mv -f /path-controller.log /path-controller.log.1
        loglen=0
    fi
}

# reuse group after restart
oldgroup=$(ip nexthop show id 1 | cut -f4 -d ' ')

declare -A client
client["100"]="192.168.123.2" # routing path through vpn-seed-server-0, vpn-shoot-0 (container vpn-shoot-s0)
client["101"]="192.168.123.3" # routing path through vpn-seed-server-0, vpn-shoot-1 (container vpn-shoot-s0)
client["110"]="192.168.124.2" # routing path through vpn-seed-server-1, vpn-shoot-0 (container vpn-shoot-s1)
client["111"]="192.168.124.3" # routing path through vpn-seed-server-1, vpn-shoot-1 (container vpn-shoot-s1)

declare -A ping_pid
declare -A ping_return

function pingAllShootClients() {
    for key in ${!client[@]}; do
        ping -W 2 -w 2 -c 1 ${client[$key]} > /dev/null &
        ping_pid[$key]=$!
    done

    for key in ${!client[@]}; do
        wait ${ping_pid[$key]}
        ping_return[$key]=$?
    done
}

function selectNewGroup() {
    local good=()
    for key in ${!client[@]}; do
        if [[ "${ping_return[$key]}" == "0" ]]; then
            good+=($key)
        fi
    done
    local len=${#good[@]}
    if (( len > 0 )); then
        # select random good path
        group=${good[$(( $RANDOM % len ))]}
    else
        # keep last value
        group=$oldgroup
    fi
}

function updateRouting() {
    # ensure nexthop configuration
    for key in ${!client[@]}; do
        ip nexthop replace id $key via ${client[$key]} dev tap$(( (key - 100) / 10 ))
    done

    log "switching from $oldgroup to $group: ip nexthop replace id 1 group $group"
    ip nexthop replace id 1 group $group

    # ensure routes
    ip route replace ${POD_NETWORK} nhid 1
    ip route replace ${SERVICE_NETWORK} nhid 1
    if [[ -n "${NODE_NETWORK}" ]]; then
      ip route replace ${NODE_NETWORK} nhid 1
    fi
    oldgroup=$group
}

while : ; do
    pingAllShootClients

    group=$oldgroup
    if [[ "$oldgroup" == "" || "${ping_return[$oldgroup]}" != "0" ]]; then
        selectNewGroup
    fi

    log "100:${client[100]}=${ping_return[100]} 101:${client[101]}=${ping_return[101]} 110:${client[110]}=${ping_return[110]} 111:${client[111]}=${ping_return[111]} old=$oldgroup new=$group"
    if [[ "$oldgroup" != "$group" ]]; then
        updateRouting
    fi
    sleep 2
done
