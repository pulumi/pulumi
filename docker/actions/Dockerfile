ARG PULUMI_VERSION=latest
FROM pulumi/pulumi:${PULUMI_VERSION}
# Extend the base pulumi/pulumi container at a specific version. "latest"
# will always correspond to the most recently released SDK.

# Label things so it lights up in GitHub Actions!
LABEL "com.github.actions.name"="Pulumi"
LABEL "com.github.actions.description"="Deploy apps and infra to your favorite cloud!"
LABEL "com.github.actions.icon"="cloud-lightning"
LABEL "com.github.actions.color"="purple"

# pulumi/actions contains documentation, examples. The actual container image is at
# https://github.com/pulumi/pulumi.
LABEL "repository"="https://github.com/pulumi/actions"
LABEL "homepage"="https://pulumi.com/docs/reference/cd-github-actions/"

# Install deps not already included in base container image.
RUN apt-get update -y && \
  apt-get install -y jq

# Copy the entrypoint script.
COPY ./entrypoint.sh /usr/bin/pulumi-action

# The app directory should contain the Pulumi program and is the pwd for the CLI.
WORKDIR /app
VOLUME ["/app"]

# We need to pass this environment variable as the confirmation to `--non-interactive` in the CLI
ENV PULUMI_SKIP_CONFIRMATIONS=true

# This image uses a thin wrapper over the Pulumi CLI as its entrypoint. As a result, you may run commands
# simply by running `docker run pulumi/pulumi up` to run the program mounted in the `/app` volume location.
ENTRYPOINT ["/usr/bin/pulumi-action", "--non-interactive"]
