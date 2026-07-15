# Program image for the run-from-program-image provider smoke test. Like Dockerfile,
# it carries the statically-linked Go program — but it also bakes the two things the
# `command` provider reads back: a toolchain binary (jq) and a workspace file.
#
# jq is the control that discriminates WHERE the provider ran. It is on this image's
# PATH and in no provider image, and a binary on PATH is precisely what the shared
# workspace mount (files, not a toolchain) cannot carry — so the provider can only
# run jq by running from this image. The marker proves less: /workspace is the shared
# volume this image seeds and every provider mounts, so a provider running from its
# own image would read the marker too. It is kept as a workspace check, split from
# the toolchain check so a failure localizes to one or the other.
FROM alpine:3
RUN apk add --no-cache jq
RUN mkdir -p /workspace && echo "hello-from-the-program-workspace" > /workspace/marker
COPY program-linux /program
ENTRYPOINT ["/program"]
