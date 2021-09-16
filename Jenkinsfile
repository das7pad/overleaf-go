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
  agent none

  stages {
    stage('Fan out') {
      parallel {
        stage('chat') {
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/chat') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/chat/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/clsi') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/clsi/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/contacts') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/contacts/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/docstore') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/docstore/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/document-updater') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/document-updater/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/filestore') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/filestore/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/linked-url-proxy') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/linked-url-proxy/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/notifications') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/notifications/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/real-time') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/real-time/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/spelling') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/spelling/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
          agent {
            label 'docker_builder'
          }
          steps {
            dir('services/web') {
              sh 'make run-ci-if-needed'
            }
            archiveArtifacts(
              allowEmptyArchive: true,
              artifacts:         'services/web/docker-image.digest.txt*',
              onlyIfSuccessful:  true,
            )
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
