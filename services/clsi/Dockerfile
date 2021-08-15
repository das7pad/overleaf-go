# Golang port of the Overleaf clsi service
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

FROM golang:1.16-alpine3.13 as build
WORKDIR /go/src/github.com/das7pad/overleaf-go/services/clsi
COPY go.mod ../../
COPY services/clsi/go.mod services/clsi/go.sum ./
RUN go mod download
COPY . ../../
RUN go build . && mv clsi /clsi

# Build the execAgent in ubuntu/debian. The texlive image will use debian.
FROM golang:1.16 as build-execAgent
WORKDIR /go/src/github.com/das7pad/overleaf-go/services/clsi
COPY . ../../
RUN go build cmd/execAgent/execAgent.go && mv execAgent /execAgent

FROM alpine:3.13
ENV SERVICE_NAME="clsi"
ENV COPY_EXEC_AGENT_SRC="/usr/local/bin/execAgent"
ENV COPY_EXEC_AGENT_DST="/mount/execAgent"
ENV SECCOMP_POLICY_PATH="/seccomp/policy.json"
ENTRYPOINT ["clsi"]
COPY services/clsi/seccomp /seccomp
COPY --from=build /clsi /usr/local/bin/
COPY --from=build-execAgent /execAgent /usr/local/bin/

RUN adduser --uid 1000 --disabled-password tex \
&&  mkdir         /cache /compiles /output \
&&  chown tex:tex /cache /compiles /output
USER tex

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
  org.opencontainers.image.licenses="AGPL-3.0-or-later" \
  org.opencontainers.image.ref.name="$RELEASE" \
  org.opencontainers.image.title="clsi" \
  org.opencontainers.image.description="REST API service for clsi"
