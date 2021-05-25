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

FROM alpine:3.13.5

RUN apk add --update bash curl openvpn iptables bc && \
    rm -rf /var/cache/apk/*

ADD shoot-client/network-connection.sh shoot-client/openvpn.config.template /

# We use "exec" and "trap 'exit' TERM" in the bash script, otherwise,
# the script won't recieve and react to SIGTERM
CMD exec /network-connection.sh
