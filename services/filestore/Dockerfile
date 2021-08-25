# Golang port of the Overleaf filestore service
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

FROM alpine:3.13
ENV SERVICE_NAME="filestore"
ENTRYPOINT ["filestore"]
COPY ./filestore /usr/local/bin/
USER nobody

ARG DATE
ARG RELEASE
ARG COMMIT
LABEL \
  org.opencontainers.image.created="$DATE" \
  org.opencontainers.image.authors="Jakob Ackermann <das7pad@outlook.com>" \
  org.opencontainers.image.url="https://github.com/das7pad/overleaf-go" \
  org.opencontainers.image.source="https://github.com/das7pad/overleaf-go" \
  org.opencontainers.image.version="$RELEASE" \
  org.opencontainers.image.revision="$COMMIT" \
  org.opencontainers.image.vendor="Jakob Ackermann" \
  org.opencontainers.image.licenses="AGPL-3.0" \
  org.opencontainers.image.ref.name="$RELEASE" \
  org.opencontainers.image.title="filestore" \
  org.opencontainers.image.description="REST API service for filestore"
