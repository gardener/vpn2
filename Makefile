# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

ENSURE_GARDENER_MOD           := $(shell go get github.com/gardener/gardener@$$(go list -m -f "{{.Version}}" github.com/gardener/gardener))
GARDENER_HACK_DIR             := $(shell go list -m -f "{{.Dir}}" github.com/gardener/gardener)/hack
VERSION                       := $(shell cat VERSION)
REPO_ROOT                     := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
HACK_DIR                      := $(REPO_ROOT)/hack
REGISTRY                      := europe-docker.pkg.dev/gardener-project/public/gardener
LOCAL_REGISTRY                := garden.local.gardener.cloud:5001
VPN_SERVER_IMAGE_REPOSITORY   := $(REGISTRY)/vpn-server
VPN_SERVER_IMAGE_TAG          := $(VERSION)
LOCAL_VPN_SERVER_IMAGE_REPO   := ${LOCAL_REGISTRY}/$(subst /,_,$(subst .,_,$(VPN_SERVER_IMAGE_REPOSITORY)))
VPN_CLIENT_IMAGE_REPOSITORY   := $(REGISTRY)/vpn-client
VPN_CLIENT_IMAGE_TAG          := $(VERSION)
LOCAL_VPN_CLIENT_IMAGE_REPO   := ${LOCAL_REGISTRY}/$(subst /,_,$(subst .,_,$(VPN_CLIENT_IMAGE_REPOSITORY)))
LD_FLAGS                      := "-w $(shell bash $(GARDENER_HACK_DIR)/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION "vpn2")"

IMAGE_TAG             := $(VERSION)
EFFECTIVE_VERSION     := $(VERSION)-$(shell git rev-parse HEAD)
ARCH                ?= amd64

PATH                          := $(GOBIN):$(PATH)

TOOLS_DIR := $(HACK_DIR)/tools
include $(GARDENER_HACK_DIR)/tools.mk

export PATH

.PHONY: tidy
tidy:
	@GO111MODULE=on go mod tidy
	@mkdir -p $(HACK_DIR) && cp $(GARDENER_HACK_DIR)/sast.sh $(HACK_DIR)/sast.sh && chmod +xw $(HACK_DIR)/sast.sh

.PHONY: vpn-server-docker-image
vpn-server-docker-image:
	@docker buildx build --platform=linux/$(ARCH) --build-arg DEBUG=$(DEBUG) -t $(VPN_SERVER_IMAGE_REPOSITORY):$(VPN_SERVER_IMAGE_TAG) -f Dockerfile --target vpn-server --rm .

.PHONY: vpn-client-docker-image
vpn-client-docker-image:
	@docker buildx build --platform=linux/$(ARCH) --build-arg DEBUG=$(DEBUG) -t $(VPN_CLIENT_IMAGE_REPOSITORY):$(VPN_CLIENT_IMAGE_TAG) -f Dockerfile --target vpn-client --rm .

.PHONY: vpn-server-to-gardener-local
vpn-server-to-gardener-local: vpn-server-docker-image
	@docker tag $(VPN_SERVER_IMAGE_REPOSITORY):$(VPN_SERVER_IMAGE_TAG) $(LOCAL_VPN_SERVER_IMAGE_REPO):$(VPN_SERVER_IMAGE_TAG)
	@docker push $(LOCAL_VPN_SERVER_IMAGE_REPO):$(VPN_SERVER_IMAGE_TAG)
	@echo "VPN server image: $(LOCAL_VPN_SERVER_IMAGE_REPO):$(VPN_SERVER_IMAGE_TAG)"

.PHONY: vpn-client-to-gardener-local
vpn-client-to-gardener-local: vpn-client-docker-image
	@docker tag $(VPN_CLIENT_IMAGE_REPOSITORY):$(VPN_CLIENT_IMAGE_TAG) $(LOCAL_VPN_CLIENT_IMAGE_REPO):$(VPN_CLIENT_IMAGE_TAG)
	@docker push $(LOCAL_VPN_CLIENT_IMAGE_REPO):$(VPN_CLIENT_IMAGE_TAG)
	@echo "VPN client image: $(LOCAL_VPN_CLIENT_IMAGE_REPO):$(VPN_CLIENT_IMAGE_TAG)"

.PHONY: docker-images
docker-images: vpn-server-docker-image vpn-client-docker-image

.PHONY: docker-images-to-gardener-local
docker-images-to-gardener-local: vpn-server-to-gardener-local vpn-client-to-gardener-local

.PHONY: release
release: docker-images docker-login docker-push

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(VPN_SERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(VPN_SERVER_IMAGE_TAG); then echo "$(VPN_SERVER_IMAGE_REPOSITORY) version $(VPN_SERVER_IMAGE_TAG) is not yet built. Please run 'make vpn-server-docker-image'"; false; fi
	@if ! docker images $(VPN_CLIENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(VPN_CLIENT_IMAGE_TAG); then echo "$(VPN_CLIENT_IMAGE_REPOSITORY) version $(VPN_CLIENT_IMAGE_TAG) is not yet built. Please run 'make vpn-client-docker-image'"; false; fi
	@gcloud docker -- push $(VPN_SERVER_IMAGE_REPOSITORY):$(VPN_SERVER_IMAGE_TAG)
	@gcloud docker -- push $(VPN_CLIENT_IMAGE_REPOSITORY):$(VPN_CLIENT_IMAGE_TAG)

.PHONY: check
check: sast-report
	go fmt ./...
	go vet ./...

# TODO(scheererj): Remove once https://github.com/gardener/gardener/pull/10642 is available as release.
TOOLS_PKG_PATH := $(shell go list -tags tools -f '{{ .Dir }}' github.com/gardener/gardener/hack/tools 2>/dev/null)
.PHONY: adjust-install-gosec.sh
adjust-install-gosec.sh:
	@chmod +xw $(TOOLS_PKG_PATH)/install-gosec.sh

.PHONY: sast
sast: adjust-install-gosec.sh $(GOSEC)
	@./hack/sast.sh

.PHONY: sast-report
sast-report: adjust-install-gosec.sh $(GOSEC)
	@./hack/sast.sh --gosec-report true

.PHONY: test
test:
	go test ./...

.PHONY: test-docker
test-docker:
	@docker run --rm --cap-add NET_ADMIN --cap-add MKNOD --privileged \
		-v $(REPO_ROOT):/src \
		-w /src \
		golang:1.26.0 \
		go test -p 1 ./...

.PHONY: build
build: build-vpn-server build-vpn-client

.PHONY: build-vpn-server
build-vpn-server:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/vpn-server  \
	    -ldflags $(LD_FLAGS)\
	    ./cmd/vpn_server/main.go

.PHONY: build-vpn-client
build-vpn-client:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/vpn-client  \
	    -ldflags $(LD_FLAGS)\
	    ./cmd/vpn_client/main.go

.PHONY: build-tunnel-controller
build-tunnel-controller:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/tunnel-controller  \
	    -ldflags $(LD_FLAGS)\
	    ./cmd/tunnel_controller/main.go
