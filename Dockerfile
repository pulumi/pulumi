# Build the image in a distinct stage so we don't need the Golang SDK.
FROM golang:1.11-stretch as builder

# Install pre-reqs.
#     - Update apt-get sources
RUN apt-get update -y
#     - Dep, for Go package management
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# Copy the source code over, restore dependencies, and get ready to build everything. We copy the Gopkg
# files explicitly first to avoid excessive rebuild times when dependencies did not change.
WORKDIR /go/src/github.com/pulumi/pulumi
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

# Install and run in Alpine Linux.
FROM debian:stretch

# Copy over the binaries built during the prior stage.
COPY --from=builder /opt/pulumi/* /usr/local/bin/

# The app directory should contain the Pulumi program and is the pwd for the CLI.
WORKDIR /app
VOLUME ["/app"]

# The app.pulumi.com access token is specified as an environment variable. You can create a new
# access token on your account page at https://app.pulumi.com/account. Please override this when
# running the Docker container using `docker run pulumi/pulumi -e "PULUMI_ACCESS_TOKEN=a1b2c2def9"`.
# ENV PULUMI_ACCESS_TOKEN

# This image uses the `pulumi` CLI as an entrypoint. As a result, you may run commands simply by
# running `docker run pulumi/pulumi up` to run the program mounted in the `/app` volume location.
ENTRYPOINT ["pulumi", "--non-interactive"]
