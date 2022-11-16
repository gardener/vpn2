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

router_id=$(ip -4 -br addr show dev bond0)
if [[ $router_id =~ ^.*[[:blank:]]+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\/[0-9]+[[:blank:]]*$ ]]; then
  router_id=${BASH_REMATCH[1]}
else
  log "unexpected ip for dev bond0: $router_id"
  exit 1
fi
log "router id: $router_id"

if [[ "$IS_SHOOT_CLIENT" == "true" ]]; then
  if [[ -n "$NODE_NETWORK" ]]; then
    route_node_network="route ${NODE_NETWORK} via \"eth0\";"
  fi
  sed -e "s|\${SERVICE_NETWORK}|${SERVICE_NETWORK}|" \
      -e "s|\${POD_NETWORK}|${POD_NETWORK}|" \
      -e "s|\${ROUTE_NODE_NETWORK}|${route_node_network}|" \
      -e "s/\${ROUTER_ID}/${router_id}/" \
      bird-shoot.conf.template > /etc/bird.conf
else
  sed -e "s/\${ROUTER_ID}/${router_id}/" \
      bird-seed.conf.template > /etc/bird.conf
fi

# start bird in foreground
exec bird -f
