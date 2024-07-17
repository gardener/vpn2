# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

GARDENER_HACK_DIR    		  := $(shell go list -m -f "{{.Dir}}" github.com/gardener/gardener)/hack
VERSION                       := $(shell cat VERSION)
REPO_ROOT                     := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
REGISTRY                      := europe-docker.pkg.dev/gardener-project/public/gardener
VPN_SERVER_IMAGE_REPOSITORY  := $(REGISTRY)/vpn-server
VPN_SERVER_IMAGE_TAG         := $(VERSION)
LOCAL_VPN_SERVER_IMAGE_REPO  := localhost:5001/$(subst /,_,$(subst .,_,$(VPN_SERVER_IMAGE_REPOSITORY)))
VPN_CLIENT_IMAGE_REPOSITORY := $(REGISTRY)/vpn-client
VPN_CLIENT_IMAGE_TAG        := $(VERSION)
LOCAL_VPN_CLIENT_IMAGE_REPO := localhost:5001/$(subst /,_,$(subst .,_,$(VPN_CLIENT_IMAGE_REPOSITORY)))
LD_FLAGS                      := "-w $(shell bash $(GARDENER_HACK_DIR)/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION "vpn2")"

IMAGE_TAG             := $(VERSION)
EFFECTIVE_VERSION     := $(VERSION)-$(shell git rev-parse HEAD)
ARCH                ?= amd64

PATH                          := $(GOBIN):$(PATH)

export PATH

.PHONY: tidy
tidy:
	@GO111MODULE=on go mod tidy

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
check:
	go fmt ./...
	go vet ./...

.PHONY: test
test:
	go test ./...

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

.PHONY: build-tunnelcontroller
build-tunnelcontroller:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/tunnelcontroller  \
	    -ldflags $(LD_FLAGS)\
	    ./cmd/tunnelcontroller/main.go
