# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

VERSION                       := $(shell cat VERSION)
REGISTRY                      := europe-docker.pkg.dev/gardener-project/public/gardener
PREFIX                        := vpn
SEED_SERVER_IMAGE_REPOSITORY  := $(REGISTRY)/$(PREFIX)-seed-server
SEED_SERVER_IMAGE_TAG         := $(VERSION)
SHOOT_CLIENT_IMAGE_REPOSITORY := $(REGISTRY)/$(PREFIX)-shoot-client
SHOOT_CLIENT_IMAGE_TAG        := $(VERSION)

PATH                          := $(GOBIN):$(PATH)

export PATH

.PHONY: seed-server-docker-image
seed-server-docker-image:
	@docker build -t $(SEED_SERVER_IMAGE_REPOSITORY):$(SEED_SERVER_IMAGE_TAG) -f seed-server/Dockerfile --rm .

.PHONY: shoot-client-docker-image
shoot-client-docker-image:
	@docker build -t $(SHOOT_CLIENT_IMAGE_REPOSITORY):$(SHOOT_CLIENT_IMAGE_TAG) -f shoot-client/Dockerfile --rm .

.PHONY: docker-images
docker-images: seed-server-docker-image shoot-client-docker-image

.PHONY: release
release: docker-images docker-login docker-push

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(SEED_SERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(SEED_SERVER_IMAGE_TAG); then echo "$(SEED_SERVER_IMAGE_REPOSITORY) version $(SEED_SERVER_IMAGE_TAG) is not yet built. Please run 'make seed-server-docker-image'"; false; fi
	@if ! docker images $(SHOOT_CLIENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(SHOOT_CLIENT_IMAGE_TAG); then echo "$(SHOOT_CLIENT_IMAGE_REPOSITORY) version $(SHOOT_CLIENT_IMAGE_TAG) is not yet built. Please run 'make shoot-client-docker-image'"; false; fi
	@docker push $(SEED_SERVER_IMAGE_REPOSITORY):$(SEED_SERVER_IMAGE_TAG)
	@docker push $(SHOOT_CLIENT_IMAGE_REPOSITORY):$(SHOOT_CLIENT_IMAGE_TAG)
