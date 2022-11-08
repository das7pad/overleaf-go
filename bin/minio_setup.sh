#!/bin/bash
# Golang port of Overleaf
# Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

set -e -o pipefail

tmp_file=$(mktemp)
trap "rm -f ${tmp_file}" EXIT

endpoint="http://${MINIO_ENDPOINT}"
if [[ "${MINIO_SECURE}" == "true" ]]; then
  endpoint="https://${MINIO_ENDPOINT}"
fi

# Setup the client with retries
for i in {1..5}; do
  mc alias set minio "${endpoint}" "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}" \
  || continue
  break
done
mc alias set minio "${endpoint}" "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}"

# Create the bucket
mc mb --ignore-existing "minio/${BUCKET}"

# Cleanup any old users
OLD=$(mc admin user list minio --json | sed -E 's/.+accessKey":"([a-f0-9]+)".+/\1/')
for user in ${OLD}; do
  mc admin user remove minio "${user}"
done
# Create the new user
# NOTE: Newly created users do not have any permissions.
mc admin user add minio "${ACCESS_KEY}" "${SECRET_KEY}"

# Grant the user access on the bucket
cat <<EOF > "$tmp_file"
$S3_POLICY
EOF
mc admin policy add minio overleaf-go "${tmp_file}"
mc admin policy set minio overleaf-go user="${ACCESS_KEY}"
