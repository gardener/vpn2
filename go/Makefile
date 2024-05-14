# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

VERSION               := $(shell cat ../VERSION)
IMAGE_TAG             := $(VERSION)
EFFECTIVE_VERSION     := $(VERSION)-$(shell git rev-parse HEAD)
GOARCH                := amd64

.PHONY: check
check:
	go fmt ./...
	go vet ./...

.PHONY: test
test:
	go test ./...

.PHONY: build
build: build-acquire-ip build-openvpn-exporter

.PHONY: build-acquire-ip
build-acquire-ip:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) GO111MODULE=on go build -o acquire-ip \
	    -ldflags "-X 'main.Version=$(EFFECTIVE_VERSION)' -X 'main.ImageTag=$(IMAGE_TAG)'"\
	    ./cmd/acquire_ip/main.go

.PHONY: build-openvpn-exporter
build-openvpn-exporter:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) GO111MODULE=on go build -o openvpn-exporter \
	    -ldflags "-X 'main.Version=$(EFFECTIVE_VERSION)' -X 'main.ImageTag=$(IMAGE_TAG)'"\
	    ./cmd/openvpn_exporter/main.go

