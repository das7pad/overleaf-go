#!/bin/bash
# Golang port of Overleaf
# Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

set -ex

SOURCE_IMAGE=${1:-texlive/texlive:TL2023-historic}
TAG=${2:-"$SOURCE_IMAGE"}

BIN=$(dirname "$0")
ROOT=$(dirname "$BIN")
TMP=$(mktemp -d)
function cleanup() {
  rm -rf "$TMP"
}
trap "cleanup" EXIT
pushd "$ROOT"

if which go > /dev/null; then
  go run "./cmd/latexmkrc-generator" > "$TMP/latexmkrc"
else
  docker run --rm -v "$ROOT:$ROOT" -w "$ROOT" golang:1.22.6 \
    go run "./cmd/latexmkrc-generator" > "$TMP/latexmkrc"
fi

docker build --pull --tag "$TAG" --file - "$TMP" <<EOF
FROM $SOURCE_IMAGE
RUN apt-get update \
 && apt-get install -y qpdf time \
 && rm -rf /var/lib/apt/lists/*
COPY ./latexmkrc /etc/latexmkrc
EOF
