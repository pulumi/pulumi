# Build the image in a distinct stage so we don't need the Golang SDK.
FROM golang:1.11-stretch as builder

# Change directories and place the minimal build scripts we need to start installing things.
WORKDIR /go/src/github.com/pulumi/pulumi
COPY ./build/ ./build/

# Install pre-reqs.
#     - Update apt-get sources
RUN apt-get update -y
#     - Dep, for Go package management
RUN . ./build/tool-versions.sh && \
    curl -L -o "$(go env GOPATH)/bin/dep" \
        https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 && \
    chmod +x "$(go env GOPATH)/bin/dep"

# Copy the source code over, restore dependencies, and get ready to build everything. We copy the Gopkg
# files explicitly first to avoid excessive rebuild times when dependencies did not change.
COPY Gopkg.* ./
RUN dep ensure -v --vendor-only
COPY . .

# Build the CLI itself.
RUN make install

# Build the plguins for each of the language hosts (Go, Node.js, Python). This just builds Go code, and
# specifically avoids building the packages, since those are distributed using package managers.
RUN cd sdk/go && make install_plugin
RUN cd sdk/nodejs && make install_plugin
RUN cd sdk/python && make install_plugin

# Install and run in Debian Stretch (to match the builder stage).
# TODO[pulumi/pulumi#1986]: consider switching to, or supporting, Alpine Linux for smaller image sizes.
FROM debian:stretch

# Install some runtime pre-reqs.
RUN apt-get update -y
RUN apt-get install -y ca-certificates curl gnupg jq

# Install the necessary runtimes.
#     - Node.js 10.x
RUN curl -sL https://deb.nodesource.com/setup_10.x | bash - && \
    apt-get install -y nodejs build-essential

# Copy the entrypoint script.
COPY ./scripts/docker-entry.sh /usr/bin/run-pulumi

# Copy over the binaries built during the prior stage.
COPY --from=builder /opt/pulumi/* /usr/bin/

# The app directory should contain the Pulumi program and is the pwd for the CLI.
WORKDIR /app
VOLUME ["/app"]

# The app.pulumi.com access token is specified as an environment variable. You can create a new
# access token on your account page at https://app.pulumi.com/account. Please override this when
# running the Docker container using `docker run pulumi/pulumi -e "PULUMI_ACCESS_TOKEN=a1b2c2def9"`.
# ENV PULUMI_ACCESS_TOKEN

# This image uses a thin wrapper over the Pulumi CLI as its entrypoint. As a result, you may run commands
# simply by running `docker run pulumi/pulumi up` to run the program mounted in the `/app` volume location.
ENTRYPOINT ["run-pulumi", "--non-interactive"]
