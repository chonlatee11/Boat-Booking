// Every stage is a `make` target so `make ci` reproduces this pipeline
// locally (D-22). Runs on the `docker` agent (docker.sock mounted) so
// testcontainers-based integration tests work (Pitfall 12, D-21).
pipeline {
    agent { label 'docker' }

    options {
        timestamps()
        disableConcurrentBuilds()
        timeout(time: 45, unit: 'MINUTES')
    }

    environment {
        // SSH sessions do not inherit the agent image's Dockerfile ENV.
        PATH = "/usr/local/go/bin:/home/jenkins/go/bin:${env.PATH}"
        TESTCONTAINERS_HOST_OVERRIDE = 'host.docker.internal'
    }

    stages {
        stage('Tools') {
            steps {
                sh 'make dev-tools'
            }
        }
        stage('Lint') {
            steps {
                sh 'make lint'
            }
        }
        stage('Unit') {
            steps {
                sh 'make test'
            }
        }
        stage('Integration') {
            steps {
                sh 'make test-integration'
            }
        }
        stage('Images') {
            steps {
                sh 'make images TAG=${GIT_COMMIT}'
            }
        }
    }
}
