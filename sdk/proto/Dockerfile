# Copyright 2016-2018, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM node:8

ENV GRPC_MONTH 09
ENV GRPC_YEAR 2018
ENV GRPC_BUILD 0593b645-15cb-4a0f-9b7f-d4958febfde3
ENV GRPC_COMMIT a07fcbcc278a8ac29a5d5ae6cd584b92d4ae49b8
ENV GRPC_ARTIFACT grpc-protoc_linux_x64-1.16.0-dev.tar.gz

# Register Microsoft key and feed
RUN apt-get update && apt-get install -y apt-transport-https
RUN set -x \
    && wget -qO- https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > microsoft.asc.gpg \
    && mv microsoft.asc.gpg /etc/apt/trusted.gpg.d/ \
    && wget -q https://packages.microsoft.com/config/debian/9/prod.list \
    && mv prod.list /etc/apt/sources.list.d/microsoft-prod.list \
    && chown root:root /etc/apt/trusted.gpg.d/microsoft.asc.gpg \
    && chown root:root /etc/apt/sources.list.d/microsoft-prod.list

RUN apt-get update \
    && apt-get install -y curl unzip golang git python-pip python-dev dotnet-sdk-2.2
RUN pip install --upgrade pip

# Install `protoc` v3.5.1.
RUN set -x \
    && curl -OL https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip \
    && unzip protoc-3.5.1-linux-x86_64.zip -d protoc3 \
    && mv protoc3/bin/* /usr/bin/ \
    && mv protoc3/include/* /usr/include/ \
    && rm protoc-3.5.1-linux-x86_64.zip

# Install Go.
RUN mkdir -p /go/src
RUN mkdir -p /go/pkg
RUN mkdir -p /go/bin
ENV GOPATH=/go
ENV PATH=$PATH:$GOPATH/bin

# Install Go protobuf tools. Use `protoc-gen-go` v1.1.0.
RUN go get -u github.com/golang/protobuf/protoc-gen-go
WORKDIR /go/src/github.com/golang/protobuf
RUN git checkout v1.1.0
RUN go install ./protoc-gen-go

# Install node gRPC tools.
RUN ln -s /usr/bin/nodejs /usr/bin/node

# NPM's grpc-tools hasn't been released in a while and we need the `minimum_node_version` flag, otherwise protoc
# emits calls to the deprecated Buffer constructor.
RUN wget https://packages.grpc.io/archive/$GRPC_YEAR/$GRPC_MONTH/$GRPC_COMMIT-$GRPC_BUILD/protoc/$GRPC_ARTIFACT
RUN mkdir -p grpc-proto
RUN tar -xzf $GRPC_ARTIFACT -C grpc-proto
RUN cp grpc-proto/protoc /usr/local/bin/grpc_tools_node_protoc
RUN cp grpc-proto/grpc_node_plugin /usr/local/bin/grpc_tools_node_protoc_plugin

# Install Python gRPC tools.
RUN python -m pip install grpcio grpcio-tools

# Install dotnet gRPC tools.
RUN set -x \ 
    && mkdir -p /packages/Grpc.Tools \
    && curl -sL https://www.nuget.org/api/v2/package/Grpc.Tools/ > /packages/Grpc.Tools/tmp.zip \
    && cd /packages/Grpc.Tools && unzip tmp.zip \
    && chmod -R a+x tools \
    && rm tmp.zip 

WORKDIR /local
