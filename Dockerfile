# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

## base
FROM alpine:3.20.1 as base

RUN apk add --update openvpn iptables iptables-legacy && \
    rm -rf /var/cache/apk/*

WORKDIR /volume

RUN mkdir -p ./bin ./sbin ./lib ./usr/bin ./usr/sbin ./usr/lib ./usr/lib/xtables ./tmp ./run ./etc/openvpn \
    ./usr/lib/openvpn/plugins ./etc/iproute2 ./etc/busybox-paths.d/busybox ./etc/logrotate.d ./etc/network/if-up.d \
    ./usr/share/iproute2 ./usr/share/udhcpc ./etc/ssl/misc ./usr/lib/engines-3 ./usr/lib/ossl-modules

RUN    cp -d /lib/ld-musl-* ./lib                                           && echo package musl \
    && cp -d /lib/libc.musl-* ./lib                                         && echo package musl \
    && cp -d /bin/busybox ./bin                                             && echo package busybox \
    && cp -d /etc/busybox-paths.d/busybox ./etc/busybox-paths.d/busybox     && echo package busybox \
    && cp -d /etc/logrotate.d/acpid ./etc/logrotate.d                       && echo package busybox \
    && cp -d /etc/network/if-up.d/dad ./etc/network/if-up.d                 && echo package busybox \
    && cp -d /etc/securetty ./etc                                           && echo package busybox \
    && cp -d /etc/udhcpc/udhcpc.conf ./etc                                  && echo package busybox \
    && cp -d /usr/share/udhcpc/default.script ./usr/share/udhcpc            && echo package busybox \
    && cp -d /bin/sh ./bin                                                  && echo package busybox-binsh \
    && cp -d /usr/lib/libcap.* ./usr/lib                                    && echo package libcap2 \
    && cp -d /usr/lib/libpsx.* ./usr/lib                                    && echo package libcap2 \
    && cp -d /usr/lib/libcap-ng.* ./usr/lib                                 && echo package libcap-ng \
    && cp -d /usr/lib/libdrop_ambient.* ./usr/lib                           && echo package libcap-ng \
    && cp -d /lib/libz.* ./lib                                              && echo package zlib \
    && cp -d /usr/lib/libzstd.* ./usr/lib                                   && echo package zstd-libs \
    && cp -d /usr/lib/libelf* ./usr/lib                                     && echo package libelf \
    && cp -d /usr/lib/libmnl.* ./usr/lib                                    && echo package libmnl \
    && cp -d /sbin/ip ./sbin                                                && echo package iproute2-minimal \
    && cp -d /usr/share/iproute2/* ./usr/share/iproute2                     && echo package iproute2-minimal \
    && cp -d -r /etc/ssl/* ./etc/ssl                                        && echo package libcrypto3 \
    && cp -d /lib/libcrypto.so.* ./lib                                      && echo package libcrypto3 \
    && cp -d /usr/lib/libcrypto.so.* ./usr/lib                              && echo package libcrypto3 \
    && cp -d /usr/lib/engines-3/* ./usr/lib/engines-3                       && echo package libcrypto3 \
    && cp -d /usr/lib/ossl-modules/* ./usr/lib/ossl-modules                 && echo package libcrypto3 \
    && cp -d /lib/libssl.so.* ./lib                                         && echo package libssl3 \
    && cp -d /usr/lib/libssl.so.* ./usr/lib                                 && echo package libssl3 \
    && cp -d /usr/lib/liblzo2.so.* ./usr/lib                                && echo package lzo \
    && cp -d /usr/lib/liblz4.so.* ./usr/lib                                 && echo package lz4-libs \
    && cp -d /usr/sbin/openvpn ./usr/sbin                                   && echo package openvpn \
    && cp -d /etc/openvpn/* ./etc/openvpn                                   && echo package openvpn \
    && cp -d /usr/lib/openvpn/plugins/openvpn* ./usr/lib/openvpn/plugins    && echo package openvpn \
    && cp -d /usr/lib/libnftnl* ./usr/lib                                   && echo package libnftnl \
    && cp -d /etc/ethertypes ./etc                                          && echo package iptables \
    && cp -d /sbin/iptables* ./sbin                                         && echo package iptables \
    && cp -d /sbin/ip6tables* ./sbin                                        && echo package iptables \
    && cp -d /sbin/xtables* ./sbin                                          && echo package iptables \
    && cp -d /usr/lib/libxtables* ./usr/lib                                 && echo package iptables \
    && cp -d /usr/lib/libip4* ./usr/lib                                     && echo package iptables \
    && cp -d /usr/lib/libip6* ./usr/lib                                     && echo package iptables \
    && cp -d /usr/lib/xtables/* ./usr/lib/xtables                           && echo package iptables

## gobuilder
FROM --platform=$BUILDPLATFORM golang:1.22.5 AS gobuilder
WORKDIR /build
COPY ./VERSION ./VERSION
COPY ./go.mod /go.sum ./
RUN go mod download
COPY ./.git ./.git
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./Makefile ./Makefile
ENV GOCACHE=/root/.cache/go-build

## gobuilder-shoot-client
FROM gobuilder AS gobuilder-shoot-client
ARG TARGETARCH
RUN --mount=type=cache,target="/root/.cache/go-build" make build-shoot-client ARCH=${TARGETARCH}

## shoot-client
FROM scratch AS shoot-client
COPY --from=base /volume /
COPY --from=gobuilder-shoot-client /build/bin/shoot-client /bin/shoot-client
ENTRYPOINT /bin/shoot-client && openvpn --config /openvpn.config

## gobuilder-seed-server
FROM gobuilder AS gobuilder-seed-server
ARG TARGETARCH
RUN --mount=type=cache,target="/root/.cache/go-build" make build-seed-server ARCH=${TARGETARCH}

## seed-server
FROM scratch AS seed-server
COPY --from=base /volume /
COPY --from=gobuilder-seed-server /build/bin/seed-server /bin/seed-server
ENTRYPOINT /bin/seed-server && openvpn --config /openvpn.config
