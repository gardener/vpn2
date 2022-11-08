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

# dummy group 2 for fallback
ip nexthop replace id 2 dev lo

oldgroup=$(ip nexthop show id 1 | cut -f4 -d ' ')
if [[ -z "$oldgroup" ]]; then
    ip nexthop add id 1 group 2
fi

declare -A client
client["10"]="192.168.123.2"
client["11"]="192.168.123.3"
client["20"]="192.168.124.2"
client["21"]="192.168.124.3"

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
    for key in ${!client[@]}; do
        if [[ "${ping_return[$key]}" == "0" ]]; then
            group=$key
            return
        fi
    done
    group="2"
}

function updateRouting() {
    # ensure nexthop configuration
    for key in ${!client[@]}; do
        ip nexthop replace id $key via ${client[$key]} dev tap$(( (key / 10) - 1 ))
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
    if [[ "${ping_return[$oldgroup]}" != "0" ]]; then
        selectNewGroup
    fi

    log "10:${client[10]}=${ping_return[10]} 11:${client[11]}=${ping_return[11]} 20:${client[20]}=${ping_return[20]} 21:${client[21]}=${ping_return[21]} old=$oldgroup new=$group"
    if [[ "$oldgroup" != "$group" ]]; then
        updateRouting
    fi
    sleep 2
done
