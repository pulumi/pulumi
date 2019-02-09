# This Dockerfile describes a base image that includes nodejs, python3,
# as well as the cli for the major clour providers Pulumi supports
# (aws, azure, gcp, kubernetes) as well as the Pulumi CLI itself.
#
# It is suitable as a general base image for an environment for deploying
# infrastructure written with Pulumi.
FROM debian:stretch

# Passing --build-arg PULUMI_VERSION=vX.Y.Z will use that version
# of the SDK. Otherwise, we use whatever get.pulumi.com thinks is
# the latest
ARG PULUMI_VERSION=latest

# Install some runtime pre-reqs.
RUN apt-get update -y
RUN apt-get install -y ca-certificates curl software-properties-common gnupg jq git

# Install the Pulumi SDK, including the CLI and language runtimes.
RUN if [ "$PULUMI_VERSION" = "latest" ]; then \
       curl -fsSL https://get.pulumi.com/ | bash; \
    else \
        curl -fsSL https://get.pulumi.com/ | bash -s -- --version $(echo $PULUMI_VERSION | cut -c 2-); \
    fi && \
    mv ~/.pulumi/bin/* /usr/bin

# Install the necessary runtimes to support Pulumi languages.
#     - Python 3
RUN apt install -y python3-pip

#     - Node.js 10.x
RUN curl -sL https://deb.nodesource.com/setup_10.x | bash - && \
    apt-get install -y nodejs build-essential

#     - Yarn
RUN curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - && \
    echo "deb https://dl.yarnpkg.com/debian/ stable main" | tee /etc/apt/sources.list.d/yarn.list && \
    apt-get install -y yarn

# Install Docker so we can build and publish containers.
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add -  && \
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable" && \
    apt-get update -y && \
    apt-get install -y docker-ce

# Install AWS IAM Authenticator, so AWS services like EKS can be used.
RUN curl -Lso /usr/bin/aws-iam-authenticator https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-07-26/bin/linux/amd64/aws-iam-authenticator && \
    chmod +x /usr/bin/aws-iam-authenticator

# Install all the cloud CLIs, so we are prepared to deploy to them.
#     - AWS
RUN pip3 install awscli --upgrade
#     - Azure
RUN echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | \
        tee /etc/apt/sources.list.d/azure-cli.list && \
    curl -L https://packages.microsoft.com/keys/microsoft.asc | apt-key add - && \
    apt-get update -y && apt-get install -y azure-cli

#     - Google Cloud
RUN echo "deb http://packages.cloud.google.com/apt cloud-sdk-$(lsb_release -c -s) main" | \
        tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
    apt-get update -y && apt-get install -y google-cloud-sdk
#     - Kubernetes
RUN curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add - && \
    echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | \
        tee -a /etc/apt/sources.list.d/kubernetes.list && \
    apt-get update -y && apt-get install -y kubectl
