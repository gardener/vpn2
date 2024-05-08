# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

############# gobuilder
FROM golang:1.22.3 AS gobuilder

WORKDIR /build
COPY ./VERSION ./VERSION
COPY ./.git ./.git
COPY ./go ./go
ARG TARGETARCH
RUN cd go; make build-acquire-ip GOARCH=$TARGETARCH

############# builder
FROM alpine:3.19.1 as builder

RUN apk add --update bash openvpn iptables iptables-legacy ncurses-libs bc && \
    rm -rf /var/cache/apk/*

WORKDIR /volume

COPY --from=gobuilder /build/go/acquire-ip ./acquire-ip

ADD shoot-client/network-connection.sh \
    shoot-client/path-controller.sh \
    ./

RUN mkdir -p ./bin ./sbin ./lib ./usr/bin ./usr/sbin ./usr/lib ./usr/lib/xtables ./usr/lib/bash ./tmp ./run ./etc/bash ./etc/openvpn ./usr/lib/openvpn/plugins ./etc/iproute2 ./etc/terminfo ./etc/logrotate.d ./etc/network/if-up.d ./usr/share/udhcpc ./etc/ssl/misc ./usr/lib/engines-1.1 \
    && cp -d /lib/ld-musl-* ./lib                                           && echo "package musl" \
    && cp -d /lib/libc.musl-* ./lib                                         && echo "package musl" \
    && cp -d /bin/busybox ./bin                                             && echo "package busybox" \
    && cp -d /etc/logrotate.d/acpid ./etc/logrotate.d                       && echo "package busybox" \
    && cp -d /etc/network/if-up.d/dad ./etc/network/if-up.d                 && echo "package busybox" \
    && cp -d /etc/securetty ./etc                                           && echo "package busybox" \
    && cp -d /etc/udhcpd.conf ./etc                                         && echo "package busybox" \
    && cp -d /usr/share/udhcpc/default.script ./usr/share/udhcpc            && echo "package busybox" \
    && cp -d /bin/sh ./bin                                                  && echo "package busybox-binsh" \
    && cp -d -r /etc/terminfo/* ./etc/terminfo                              && echo "package ncurses-terminfo-base" \
    && cp -d /usr/lib/libformw.so.* ./usr/lib                               && echo "package ncurses-libs" \
    && cp -d /usr/lib/libmenuw.so.* ./usr/lib                               && echo "package ncurses-libs" \
    && cp -d /usr/lib/libncursesw.so.* ./usr/lib                            && echo "package ncurses-libs" \
    && cp -d /usr/lib/libpanelw.so.* ./usr/lib                              && echo "package ncurses-libs" \
    && cp -d /usr/lib/libreadline.so.* ./usr/lib                            && echo "package readline" \
    && cp -d /etc/inputrc ./etc                                             && echo "package readline" \
    && cp -d /bin/bash ./bin                                                && echo "package bash" \
    && cp -d /etc/bash/bashrc ./etc/bash                                    && echo "package bash" \
    && cp -d /usr/lib/bash/* ./usr/lib/bash                                 && echo "package bash" \
    && cp -d /bin/cat ./bin                                                 && echo "package coreutils" \
    && cp -d /bin/date ./bin                                                && echo "package coreutils" \
    && cp -d /bin/echo ./bin                                                && echo "package coreutils" \
    && cp -d /bin/sleep ./bin                                               && echo "package coreutils" \
    && cp -d /bin/ping ./bin                                                && echo "package iputils" \
    && cp -d /usr/bin/cut ./usr/bin                                         && echo "package coreutils" \
    && cp -d /usr/bin/bc ./usr/bin                                          && echo "package bc" \
    && cp -d /bin/sed ./bin                                                 && echo "package sed" \
    && cp -d /usr/lib/libcap.* ./usr/lib                                    && echo "package libcap" \
    && cp -d /usr/lib/libpsx.* ./usr/lib                                    && echo "package libcap" \
    && cp -d /usr/lib/libcap-ng.* ./usr/lib                                 && echo "package libcap-ng-dev" \
    && cp -d /lib/libz.* ./lib                                              && echo "package zlib" \
    && cp -d /usr/lib/libzstd.* ./usr/lib                                   && echo "package zstd-libs" \
    && cp -d /usr/lib/libelf* ./usr/lib                                     && echo "package libelf" \
    && cp -d /usr/lib/libmnl.* ./usr/lib                                    && echo "package libmnl" \
    && cp -d /sbin/ip ./sbin                                                && echo "package iproute2-minimal" \
    && cp -d -r /etc/ssl/* ./etc/ssl                                        && echo "package libcrypto1.1" \
    && cp -d /lib/libcrypto.so.* ./lib                                      && echo "package libcrypto1.1" \
    && cp -d /usr/lib/libcrypto.so.* ./usr/lib                              && echo "package libcrypto1.1" \
    && cp -d /lib/libssl.so.* ./lib                                         && echo "package libssl1.1" \
    && cp -d /usr/lib/libssl.so.* ./usr/lib                                 && echo "package libssl1.1" \
    && cp -d /usr/lib/liblzo2.so.* ./usr/lib                                && echo "package lzo" \
    && cp -d /usr/lib/liblz4.so.* ./usr/lib                                 && echo "package openvpn" \
    && cp -d /usr/sbin/openvpn ./usr/sbin                                   && echo "package openvpn" \
    && cp -d /etc/openvpn/* ./etc/openvpn                                   && echo "package openvpn" \
    && cp -d /usr/lib/openvpn/plugins/openvpn* ./usr/lib/openvpn/plugins    && echo "package openvpn" \
    && cp -d /usr/lib/libnftnl* ./usr/lib                                   && echo "package libnftnl" \
    && cp -d /etc/ethertypes ./etc                                          && echo "package iptables" \
    && cp -d /sbin/iptables* ./sbin                                         && echo "package iptables" \
    && cp -d /sbin/xtables* ./sbin                                          && echo "package iptables" \
    && cp -d /usr/lib/libxtables* ./usr/lib                                 && echo "package iptables" \
    && cp -d /usr/lib/libip4* ./usr/lib                                     && echo "package iptables" \
    && cp -d /usr/lib/libip6* ./usr/lib                                     && echo "package iptables" \
    && cp -d /usr/lib/xtables/* ./usr/lib/xtables                           && echo "package iptables" \
    && cp -d /sbin/ip6tables* ./sbin                                        && echo "package ip6tables"

############# vpn-shoot-client
FROM scratch

COPY --from=builder /volume /

# We use "exec" and "trap 'exit' TERM" in the bash script, otherwise,
# the script won't receive and react to SIGTERM
CMD exec /network-connection.sh
