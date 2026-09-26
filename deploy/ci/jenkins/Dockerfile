# Jenkins controller: JCasC-configured, no anonymous access, 0 executors (all
# builds run on the docker-agent so the controller never touches docker.sock)
# (D-21).
FROM jenkins/jenkins:2.568.3-lts-jdk21

COPY plugins.txt /usr/share/jenkins/ref/plugins.txt
RUN jenkins-plugin-cli --plugin-file /usr/share/jenkins/ref/plugins.txt

COPY casc.yaml /usr/local/jenkins-casc.yaml

ENV CASC_JENKINS_CONFIG=/usr/local/jenkins-casc.yaml \
    JAVA_OPTS="-Djenkins.install.runSetupWizard=false -Dhudson.plugins.git.GitSCM.ALLOW_LOCAL_CHECKOUT=true"
