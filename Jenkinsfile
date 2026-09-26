// Every stage is a `make` target so `make ci` reproduces this pipeline
// locally (D-22). Runs on the `docker` agent (docker.sock mounted) so
// testcontainers-based integration tests work (Pitfall 12, D-21). Stage
// order mirrors the Makefile `ci` target exactly; only Push is gated
// (branch 'main').
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
        stage('Prepare') {
            steps {
                script {
                    def resolvedBase = null
                    if (env.BRANCH_NAME == 'main' && env.GIT_PREVIOUS_SUCCESSFUL_COMMIT?.trim()) {
                        resolvedBase = env.GIT_PREVIOUS_SUCCESSFUL_COMMIT
                    } else {
                        sh(script: 'git fetch origin main', returnStatus: true)
                        if (sh(script: 'git rev-parse --verify origin/main', returnStatus: true) == 0) {
                            resolvedBase = 'origin/main'
                        }
                    }
                    // changed-services.sh treats any base-ref that fails
                    // `git rev-parse --verify` as "select all services" --
                    // the safe default when neither GIT_PREVIOUS_SUCCESSFUL_COMMIT
                    // nor origin/main resolves.
                    env.BASE = resolvedBase ?: 'unresolved-base-select-all'
                    echo "Diff base for changed-services.sh: ${env.BASE}"
                }
            }
        }
        stage('Tools') {
            steps {
                sh 'make dev-tools'
                sh 'npm --prefix apps/web ci'
            }
        }
        stage('Lint') {
            steps {
                sh 'make lint'
            }
        }
        stage('Proto') {
            steps {
                sh 'make proto-check'
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
        stage('Migrations') {
            steps {
                sh 'make migrate-validate'
            }
        }
        stage('Template') {
            steps {
                sh 'make template-smoke'
            }
        }
        stage('Web') {
            steps {
                sh 'make web-check'
            }
        }
        stage('Images') {
            steps {
                sh 'make images BASE=$BASE TAG=${GIT_COMMIT}'
            }
        }
        stage('Push') {
            when { branch 'main' }
            steps {
                withCredentials([usernamePassword(credentialsId: 'harbor', usernameVariable: 'HARBOR_USER', passwordVariable: 'HARBOR_PASSWORD')]) {
                    sh 'make push BASE=$BASE TAG=${GIT_COMMIT}'
                }
            }
        }
    }
}
