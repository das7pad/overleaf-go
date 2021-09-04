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
        stage('chat') {
          when {
            beforeAgent true
            changeset "services/chat/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/chat') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/chat/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/chat') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('clsi') {
          when {
            beforeAgent true
            changeset "services/clsi/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/clsi') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/clsi/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/clsi') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('contacts') {
          when {
            beforeAgent true
            changeset "services/contacts/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/contacts') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/contacts/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/contacts') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('docstore') {
          when {
            beforeAgent true
            changeset "services/docstore/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/docstore') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/docstore/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/docstore') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('document-updater') {
          when {
            beforeAgent true
            changeset "services/document-updater/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/document-updater') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/document-updater/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/document-updater') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('filestore') {
          when {
            beforeAgent true
            changeset "services/filestore/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/filestore') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/filestore/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/filestore') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('linked-url-proxy') {
          when {
            beforeAgent true
            changeset "services/linked-url-proxy/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/linked-url-proxy') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/linked-url-proxy/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/linked-url-proxy') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('notifications') {
          when {
            beforeAgent true
            changeset "services/notifications/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/notifications') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/notifications/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/notifications') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('real-time') {
          when {
            beforeAgent true
            changeset "services/real-time/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/real-time') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/real-time/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/real-time') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('spelling') {
          when {
            beforeAgent true
            changeset "services/spelling/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/spelling') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/spelling/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/spelling') {
                sh 'make docker/clean'
              }
            }
          }
        }
        stage('web') {
          when {
            beforeAgent true
            changeset "services/web/**"
          }
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/web') {
              sh 'make docker/build/production'
              sh 'make docker/push'
            }
            archiveArtifacts 'services/web/docker-image.digest.txt'
          }
          post {
            cleanup {
              dir('services/web') {
                sh 'make docker/clean'
              }
            }
          }
        }
      }
    }
  }
}
