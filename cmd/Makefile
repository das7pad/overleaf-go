# Golang port of Overleaf
# Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published
# by the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

BINS = $(filter-out internal Makefile Dockerfile bin.tar,$(wildcard *))
PWD ?= $(realpath .)
ROOT = $(abspath $(PWD)/../)
PACKAGE ?= github.com/das7pad/overleaf-go
ROOT_DOCKER = /go/src/$(PACKAGE)
PWD_DOCKER = $(ROOT_DOCKER)
BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD)
BUILD_NUMBER ?= local
GIT_COMMIT ?= $(shell git rev-parse HEAD)
GIT_PREVIOUS_SUCCESSFUL_COMMIT ?= origin/main
RELEASE ?= $(shell git describe --tags --always | sed 's/-g/-/;s/^v//')
DOCKER_REGISTRY ?= localhost:5000

IMAGE_BARE = $(DOCKER_REGISTRY)/sharelatex/cmd
IMAGE = $(IMAGE_BARE):$(BRANCH_NAME)-$(BUILD_NUMBER)

BUILD_IN = \
	docker run \
 		--rm \
	 	--volume $(ROOT):$(ROOT_DOCKER) \
	 	--volume $(ROOT)/.cache/go-mod:/go/pkg/mod \
	 	--workdir $(PWD_DOCKER) \
	 	--env HOME=$(ROOT_DOCKER)/.cache/home/ \
	 	--user $(shell id -u)

run-ci-if-needed:
	rm -f docker-image.digest.txt
	$(MAKE) check-not-changed || $(MAKE) run-ci

run-ci:
	$(MAKE) docker/build/production
	$(MAKE) docker/push

check-not-changed:
	$(BUILD_IN) golang:1.17-alpine3.14 \
 		go list -f '{{ join .Deps "\n" }}' $(addprefix ./cmd/,$(BINS)) \
	| grep $(PACKAGE) | sed s,$(PACKAGE)/,, \
	| xargs echo \
	 	go.mod \
	 	go.sum \
	 	cmd/ \
	| sort | uniq \
	| xargs git -C ../ diff --exit-code --name-only $(GIT_PREVIOUS_SUCCESSFUL_COMMIT)

.PHONY: $(BINS)
$(BINS):
	$(BUILD_IN) golang:1.17-alpine3.14 sh -c 'go build ./cmd/$@'

bin.tar: $(BINS)
	tar  --sort=name --mtime=2021-01-01T00:00Z -C ../ -f $@ -c $(BINS)

docker/build/production: bin.tar
	docker build \
		--tag $(IMAGE) \
		--build-arg COMMIT=$(GIT_COMMIT) \
		--build-arg DATE=$(shell date --rfc-3339=s | sed 's/ /T/') \
		--build-arg RELEASE=$(RELEASE) \
		--file Dockerfile \
		.

docker/push:
	docker push $(IMAGE)
	$(MAKE) log_image_digest

log_image_digest:
	docker inspect -f '{{ index .RepoDigests 0 }}' $(IMAGE) \
	| cut -d: -f2 \
	> docker-image.digest.txt

docker/clean:
	rm -f bin.tar $(addprefix ../,$(BINS))
	docker rmi \
		$(IMAGE) \
		--force