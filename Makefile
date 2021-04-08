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

VERSION                       := $(shell cat VERSION)
REGISTRY                      := eu.gcr.io/gardener-project/gardener
PREFIX                        := vpn
SEED_CLIENT_IMAGE_REPOSITORY  := $(REGISTRY)/$(PREFIX)-seed-client
SEED_CLIENT_IMAGE_TAG         := $(VERSION)
SEED_SERVER_IMAGE_REPOSITORY  := $(REGISTRY)/$(PREFIX)-seed-server
SEED_SERVER_IMAGE_TAG         := $(VERSION)
SHOOT_CLIENT_IMAGE_REPOSITORY := $(REGISTRY)/$(PREFIX)-shoot-client
SHOOT_CLIENT_IMAGE_TAG        := $(VERSION)

PATH                          := $(GOBIN):$(PATH)

export PATH

.PHONY: seed-client-docker-image
seed-client-docker-image:
	@docker build -t $(SEED_CLIENT_IMAGE_REPOSITORY):$(SEED_CLIENT_IMAGE_TAG) -f seed-client/Dockerfile --rm .

.PHONY: seed-server-docker-image
seed-server-docker-image:
	@docker build -t $(SEED_SERVER_IMAGE_REPOSITORY):$(SEED_SERVER_IMAGE_TAG) -f seed-server/Dockerfile --rm .

.PHONY: shoot-client-docker-image
shoot-client-docker-image:
	@docker build -t $(SHOOT_CLIENT_IMAGE_REPOSITORY):$(SHOOT_CLIENT_IMAGE_TAG) -f shoot-client/Dockerfile --rm .

.PHONY: docker-images
docker-images: seed-client-docker-image seed-server-docker-image shoot-client-docker-image

.PHONY: release
release: docker-images docker-login docker-push

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(SEED_CLIENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(SEED_CLIENT_IMAGE_TAG); then echo "$(SEED_CLIENT_IMAGE_REPOSITORY) version $(SEED_CLIENT_IMAGE_TAG) is not yet built. Please run 'make seed-client-docker-image'"; false; fi
	@if ! docker images $(SEED_SERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(SEED_SERVER_IMAGE_TAG); then echo "$(SEED_SERVER_IMAGE_REPOSITORY) version $(SEED_SERVER_IMAGE_TAG) is not yet built. Please run 'make seed-server-docker-image'"; false; fi
	@if ! docker images $(SHOOT_CLIENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(SHOOT_CLIENT_IMAGE_TAG); then echo "$(SHOOT_CLIENT_IMAGE_REPOSITORY) version $(SHOOT_CLIENT_IMAGE_TAG) is not yet built. Please run 'make shoot-client-docker-image'"; false; fi
	@gcloud docker -- push $(SEED_CLIENT_IMAGE_REPOSITORY):$(SEED_CLIENT_IMAGE_TAG)
	@gcloud docker -- push $(SEED_SERVER_IMAGE_REPOSITORY):$(SEED_SERVER_IMAGE_TAG)
	@gcloud docker -- push $(SHOOT_CLIENT_IMAGE_REPOSITORY):$(SHOOT_CLIENT_IMAGE_TAG)
