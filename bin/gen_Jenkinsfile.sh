#!/bin/bash

exec > Jenkinsfile

cat <<EOF
// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

pipeline {
  agent {
    label 'non_docker_builder'
  }

  stages {
    stage('Fan out') {
      parallel {
EOF

for path in services/*/; do
  service=${path%/}
  serviceName=${service#services/}
  cat <<EOF
        stage('${serviceName}') {
          when {
            beforeAgent true
            changeset "${service}/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('${service}') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts '${service}/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('${service}') {
                sh 'make docker/clean'
              }
            }
          }
        }
EOF
done

cat <<EOF
      }
    }
  }
}
EOF
