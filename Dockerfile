FROM golang:1.16 as builder

ARG ACCESS_TOKEN_USR="nothing"
ARG ACCESS_TOKEN_PWD="nothing"

# git is required to fetch go dependencies
RUN printf "machine github.com\n\
    login ${ACCESS_TOKEN_USR}\n\
    password ${ACCESS_TOKEN_PWD}\n\
    \n\
    machine api.github.com\n\
    login ${ACCESS_TOKEN_USR}\n\
    password ${ACCESS_TOKEN_PWD}\n"\
    >> /root/.netrc

RUN chmod 600 /root/.netrc

RUN mkdir /workspace
WORKDIR workspace

COPY refresher/ /workspace/refresher/
COPY pkg/ /workspace/pkg/
COPY sdk/ /workspace/sdk/
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
WORKDIR /workspace/refresher

RUN go mod download

# Copy the go source
COPY .. .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o pulumiMapperConsumer refresher/consumer/main.go

FROM alpine:3.13.1

WORKDIR /

COPY --from=builder /workspace/refresher/pulumiMapperConsumer .

ENTRYPOINT ["/pulumiMapperConsumer"]