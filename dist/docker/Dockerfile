FROM python:3.7-slim-stretch
# TODO[pulumi/pulumi#1986]: consider switching to, or supporting, Alpine Linux for smaller image sizes.

LABEL "repository"="https://github.com/pulumi/pulumi"
LABEL "homepage"="https://pulumi.com/"
LABEL "maintainer"="Pulumi Team <team@pulumi.com>"

# Install deps all in one step
RUN apt-get update -y && \
  apt-get install -y \
    apt-transport-https \
    build-essential \
    ca-certificates \
    curl \
    git \
    gnupg \
    software-properties-common \
  && \
  # Get all of the signatures we need all at once
  curl -fsSL https://deb.nodesource.com/gpgkey/nodesource.gpg.key | apt-key add - && \
  curl -fsSL https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - && \
  curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add && \
  curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
  curl -fsSL https://packages.microsoft.com/keys/microsoft.asc | apt-key add - && \
  # IAM Authenticator for EKS
  curl -fsSLo /usr/bin/aws-iam-authenticator https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-07-26/bin/linux/amd64/aws-iam-authenticator && \
  chmod +x /usr/bin/aws-iam-authenticator && \
  # Add additional apt repos all at once
  echo "deb https://deb.nodesource.com/node_11.x $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/node.list && \
  echo "deb https://dl.yarnpkg.com/debian/ stable main" | tee /etc/apt/sources.list.d/yarn.list && \
  echo "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list && \
  echo "deb http://packages.cloud.google.com/apt cloud-sdk-$(lsb_release -cs) main" | tee /etc/apt/sources.list.d/google-cloud-sdk.list && \
  echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list &&\
  echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/azure.list && \
  # Install second wave of dependencies
  apt-get update -y && \
    apt-get install -y \
    azure-cli \
    docker-ce \
    google-cloud-sdk \
    kubectl \
    nodejs \
    yarn \
  && \
  pip install awscli --upgrade && \
  # Clean up the lists work
  rm -rf /var/lib/apt/lists/*

# Install Helm, pull default repository packages
RUN curl -s https://get.helm.sh/helm-v2.14.3-linux-amd64.tar.gz | tar -xvz -C /tmp && \
    mv /tmp/linux-amd64/helm /bin && \
    rm -Rf /tmp/linux-amd64 && \
    helm init --client-only

# Passing --build-arg PULUMI_VERSION=vX.Y.Z will use that version
# of the SDK. Otherwise, we use whatever get.pulumi.com thinks is
# the latest
ARG PULUMI_VERSION=latest

# Install the Pulumi SDK, including the CLI and language runtimes.
RUN if [ "$PULUMI_VERSION" = "latest" ]; then \
    curl -fsSL https://get.pulumi.com/ | bash; \
  else \
    curl -fsSL https://get.pulumi.com/ | bash -s -- --version $(echo $PULUMI_VERSION | cut -c 2-); \
  fi && \
  mv ~/.pulumi/bin/* /usr/bin

# I think it's safe to say if we're using this mega image, we want pulumi
ENTRYPOINT ["pulumi"]
