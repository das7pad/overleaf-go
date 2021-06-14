# Golang port of the Overleaf real-time service
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

IMAGE_BARE = $(DOCKER_REGISTRY)/sharelatex/real-time
IMAGE = $(IMAGE_BARE):$(BRANCH_NAME)-$(BUILD_NUMBER)

docker/build:
	docker build \
		--tag $(IMAGE)-build \
		--target build \
		.

docker/build/production: docker/build
	docker build \
		--tag $(IMAGE) \
		--cache-from $(IMAGE)-build \
		--build-arg COMMIT=$(GIT_COMMIT) \
		--build-arg DATE=$(shell date --rfc-3339=s | sed 's/ /T/') \
		--build-arg RELEASE=$(RELEASE) \
		.

docker/push:
	docker push $(IMAGE)

log_image_digest:
	docker inspect -f '{{ index .RepoDigests 0 }}' $(IMAGE) \
	| cut -d: -f2 \
	> docker-image.digest.txt

docker/clean:
	docker rmi \
		$(IMAGE) \
		$(IMAGE)-build \
		--force

.PHONY: public.tar.gz
public.tar.gz:
	echo "window.io='plain'" > socket.io.min.js
	tar --create --sort=name --mtime=2021-01-01T00:00Z socket.io.min.js \
	| gzip -6 \
	> public.tar.gz
	sha256sum public.tar.gz | awk '{ print $$1 }' > public.tar.gz.checksum.txt
	touch -m -d 2021-01-01T00:00Z public.tar.gz public.tar.gz.checksum.txt

clean/public.tar.gz:
	rm -f public.tar.gz public.tar.gz.checksum.txt socket.io.min.js

.PHONY: test
