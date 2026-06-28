# The program image: compile the Go program and run the binary directly. A Go Pulumi
# program needs NO bootstrap shim — the Go SDK reads the monitor/engine address from the
# PULUMI_* environment the OCI host sets, so the compiled binary is the entrypoint. This
# is yours to edit — use any Go/base images you like; the only contract is that the
# container runs the program when started.
FROM golang:1.25-alpine AS build
WORKDIR /app
# -mod=mod lets go resolve and record dependencies (incl. any SDK declared as a local
# `replace` by `pulumi package add`) without a pre-existing go.sum.
ENV GOFLAGS=-mod=mod
# Download deps in their own cached layer, before the program. `pulumi package add` writes
# SDKs under sdks/ and a `replace => ./sdks/...` into go.mod; copying go.mod + sdks/ here
# lets `go mod download` resolve the local replace. sdks/ is shipped (with .gitkeep) so this
# COPY is a no-op when no SDK has been added. This layer is invalidated only when deps/SDKs
# change — not on every program edit.
COPY go.mod ./
COPY sdks ./sdks
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /program .

FROM alpine:3
COPY --from=build /program /program
ENTRYPOINT ["/program"]
