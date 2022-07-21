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

FROM alpine:3.15.4 as builder

RUN apk add --update bash openvpn bc iptables && \
    rm -rf /var/cache/apk/*

WORKDIR /volume

ADD seed-server/network-connection.sh seed-server/openvpn.config.template seed-server/firewall.sh ./

RUN mkdir -p ./client-config-dir
ADD seed-server/vpn-shoot-client.template ./client-config-dir

RUN mkdir -p ./bin ./sbin ./lib ./usr/bin ./usr/sbin ./usr/lib ./usr/lib/xtables ./tmp ./run \
    && cp -d /lib/ld-musl-* ./lib \
    && cp -d /lib/libc.musl-* ./lib \
    && cp -d /lib/libcrypto.so.* ./usr/lib \
    && cp -d /lib/libssl.so.* ./usr/lib \
    && cp -d /lib/libz.so.* ./lib \
    && cp -d /usr/lib/libelf* ./usr/lib \
    && cp -d /usr/lib/libip4* ./usr/lib \
    && cp -d /usr/lib/libip6* ./usr/lib \
    && cp -d /usr/lib/liblzo2.so.* ./usr/lib \
    && cp -d /usr/lib/libmnl* ./usr/lib \
    && cp -d /usr/lib/libncursesw.so.* ./usr/lib \
    && cp -d /usr/lib/libnftnl* ./usr/lib \
    && cp -d /usr/lib/libreadline.so.* ./usr/lib \
    && cp -d /usr/lib/libxtables* ./usr/lib \
    && cp -d /usr/lib/xtables/* ./usr/lib/xtables \
    && cp -d /bin/bash ./bin \
    && cp -d /bin/busybox ./bin \
    && cp -d /bin/cat ./bin \
    && cp -d /bin/date ./bin \
    && cp -d /bin/echo ./bin \
    && cp -d /bin/grep ./bin \
    && cp -d /bin/sed ./bin \
    && cp -d /bin/sh ./bin \
    && cp -d /sbin/ip ./sbin \
    && cp -d /sbin/iptables* ./sbin \
    && cp -d /sbin/xtables* ./sbin \
    && cp -d /usr/bin/bc ./usr/bin \
    && cp -d /usr/bin/cut ./usr/bin \
    && cp -d /usr/sbin/openvpn ./usr/sbin

FROM scratch

COPY --from=builder /volume /

EXPOSE 1194

# We use "exec" and "trap 'exit' TERM" in the bash script, otherwise,
# the script won't recieve and react to SIGTERM
CMD exec /network-connection.sh
