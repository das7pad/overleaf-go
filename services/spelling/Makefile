# Golang port of the Overleaf spelling service
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

BRANCH_NAME ?= $(shell git rev-parse --abbrev-ref HEAD)
BUILD_NUMBER ?= local
GIT_COMMIT ?= $(shell git rev-parse HEAD)
RELEASE ?= $(shell git describe --tags --always | sed 's/-g/-/;s/^v//')
DOCKER_REGISTRY ?= localhost:5000

IMAGE_BARE = $(DOCKER_REGISTRY)/sharelatex/spelling
IMAGE = $(IMAGE_BARE):$(BRANCH_NAME)-$(BUILD_NUMBER)
IMAGE_ASPELL_CACHE = $(IMAGE_BARE):go-aspell-cache

docker/build:
	docker build \
		--tag $(IMAGE)-build \
		--target build \
		--file Dockerfile \
		../../

docker/pull/apell-cache:
	docker pull $(IMAGE_ASPELL_CACHE) || true
	docker tag $(IMAGE_ASPELL_CACHE) $(IMAGE)-aspell-cache || true

docker/build/production: docker/pull/apell-cache
docker/build/production: docker/build
	docker build \
		--tag $(IMAGE_ASPELL_CACHE) \
		--cache-from $(IMAGE)-build \
		--cache-from $(IMAGE)-aspell-cache \
		--target aspell-cache \
		--file Dockerfile \
		../../

	docker build \
		--tag $(IMAGE) \
		--cache-from $(IMAGE)-build \
		--cache-from $(IMAGE_ASPELL_CACHE) \
		--build-arg COMMIT=$(GIT_COMMIT) \
		--build-arg DATE=$(shell date --rfc-3339=s | sed 's/ /T/') \
		--build-arg RELEASE=$(RELEASE) \
		--file Dockerfile \
		../../

docker/push:
	docker push $(IMAGE_ASPELL_CACHE)
	docker push $(IMAGE)
	$(MAKE) log_image_digest

log_image_digest:
	docker inspect -f '{{ index .RepoDigests 0 }}' $(IMAGE) \
	| cut -d: -f2 \
	> docker-image.digest.txt

docker/clean:
	docker rmi \
		$(IMAGE) \
		$(IMAGE)-build \
		$(IMAGE)-aspell-cache \
		$(IMAGE_ASPELL_CACHE) \
		--force

.PHONY: test
