# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

GARDENER_HACK_DIR    		  := $(shell go list -m -f "{{.Dir}}" github.com/gardener/gardener)/hack
VERSION                       := $(shell cat VERSION)
REPO_ROOT                     := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
REGISTRY                      := europe-docker.pkg.dev/gardener-project/public/gardener
PREFIX                        := vpn
SEED_SERVER_IMAGE_REPOSITORY  := $(REGISTRY)/$(PREFIX)-seed-server
SEED_SERVER_IMAGE_TAG         := $(VERSION)
LOCAL_SEED_SERVER_IMAGE_REPO  := localhost:5001/$(subst /,_,$(subst .,_,$(SEED_SERVER_IMAGE_REPOSITORY)))
SHOOT_CLIENT_IMAGE_REPOSITORY := $(REGISTRY)/$(PREFIX)-shoot-client
SHOOT_CLIENT_IMAGE_TAG        := $(VERSION)
LOCAL_SHOOT_CLIENT_IMAGE_REPO := localhost:5001/$(subst /,_,$(subst .,_,$(SHOOT_CLIENT_IMAGE_REPOSITORY)))
LD_FLAGS                      := "-w $(shell bash $(GARDENER_HACK_DIR)/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION "vpn2")"

IMAGE_TAG             := $(VERSION)
EFFECTIVE_VERSION     := $(VERSION)-$(shell git rev-parse HEAD)
ARCH                := amd64

PATH                          := $(GOBIN):$(PATH)

export PATH

.PHONY: tidy
tidy:
	@GO111MODULE=on go mod tidy

.PHONY: seed-server-docker-image
seed-server-docker-image:
	@docker build --platform=linux/$(ARCH) -t $(SEED_SERVER_IMAGE_REPOSITORY):$(SEED_SERVER_IMAGE_TAG) -f Dockerfile --target seed-server --rm .

.PHONY: shoot-client-docker-image
shoot-client-docker-image:
	@docker build --platform=linux/$(ARCH) -t $(SHOOT_CLIENT_IMAGE_REPOSITORY):$(SHOOT_CLIENT_IMAGE_TAG) -f Dockerfile --target shoot-client --rm .

.PHONY: seed-server-to-gardener-local
seed-server-to-gardener-local: seed-server-docker-image
	@docker tag $(SEED_SERVER_IMAGE_REPOSITORY):$(SEED_SERVER_IMAGE_TAG) $(LOCAL_SEED_SERVER_IMAGE_REPO):$(SEED_SERVER_IMAGE_TAG)
	@docker push $(LOCAL_SEED_SERVER_IMAGE_REPO):$(SEED_SERVER_IMAGE_TAG)
	@echo "seed server image: $(LOCAL_SEED_SERVER_IMAGE_REPO):$(SEED_SERVER_IMAGE_TAG)"

.PHONY: shoot-client-to-gardener-local
shoot-client-to-gardener-local: shoot-client-docker-image
	@docker tag $(SHOOT_CLIENT_IMAGE_REPOSITORY):$(SHOOT_CLIENT_IMAGE_TAG) $(LOCAL_SHOOT_CLIENT_IMAGE_REPO):$(SHOOT_CLIENT_IMAGE_TAG)
	@docker push $(LOCAL_SHOOT_CLIENT_IMAGE_REPO):$(SHOOT_CLIENT_IMAGE_TAG)
	@echo "shoot client image: $(LOCAL_SHOOT_CLIENT_IMAGE_REPO):$(SHOOT_CLIENT_IMAGE_TAG)"

.PHONY: docker-images
docker-images: seed-server-docker-image shoot-client-docker-image

.PHONY: docker-images-to-gardener-local
docker-images-to-gardener-local: seed-server-to-gardener-local shoot-client-to-gardener-local

.PHONY: release
release: docker-images docker-login docker-push

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(SEED_SERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(SEED_SERVER_IMAGE_TAG); then echo "$(SEED_SERVER_IMAGE_REPOSITORY) version $(SEED_SERVER_IMAGE_TAG) is not yet built. Please run 'make seed-server-docker-image'"; false; fi
	@if ! docker images $(SHOOT_CLIENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(SHOOT_CLIENT_IMAGE_TAG); then echo "$(SHOOT_CLIENT_IMAGE_REPOSITORY) version $(SHOOT_CLIENT_IMAGE_TAG) is not yet built. Please run 'make shoot-client-docker-image'"; false; fi
	@gcloud docker -- push $(SEED_SERVER_IMAGE_REPOSITORY):$(SEED_SERVER_IMAGE_TAG)
	@gcloud docker -- push $(SHOOT_CLIENT_IMAGE_REPOSITORY):$(SHOOT_CLIENT_IMAGE_TAG)

.PHONY: check
check:
	go fmt ./...
	go vet ./...

.PHONY: test
test:
	go test ./...

.PHONY: build
build: build-seed-server build-shoot-client

.PHONY: build-seed-server
build-seed-server:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/seed-server  \
	    -ldflags $(LD_FLAGS)\
	    ./cmd/seed_server/main.go

.PHONY: build-shoot-client
build-shoot-client:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/shoot-client  \
	    -ldflags $(LD_FLAGS)\
	    ./cmd/shoot_client/main.go
