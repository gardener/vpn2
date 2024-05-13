# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

## base
FROM --platform=$TARGETPLATFORM cgr.dev/chainguard/wolfi-base as base
RUN apk update && \
    apk add openvpn ip6tables iptables && \
    rm $(which apk) && \
    rm -r /var/cache/apk

## gobuilder
FROM --platform=$BUILDPLATFORM golang:1.22.3 AS gobuilder

WORKDIR /build
COPY ./VERSION ./VERSION
COPY ./go.mod /go.sum ./
RUN go mod download
COPY ./.git ./.git
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./Makefile ./Makefile

## gobuilder-shoot-client
FROM gobuilder AS gobuilder-shoot-client

ARG TARGETARCH
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" make build-shoot-client GOARCH=$TARGETARCH


## shoot-client
FROM base AS shoot-client
COPY --from=gobuilder-shoot-client /build/bin/shoot-client /bin/shoot-client
ENTRYPOINT [ "/bin/shoot-client" ]

## gobuilder-seed-server
FROM gobuilder AS gobuilder-seed-server

ARG TARGETARCH
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" make build-seed-server GOARCH=$TARGETARCH


## shoot-client
FROM base AS seed-server
COPY --from=gobuilder-seed-server /build/bin/seed-server /bin/seed-server
ENTRYPOINT [ "/bin/seed-server" ]
