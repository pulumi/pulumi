# Program image for the run-from-program-image provider smoke test. Like Dockerfile,
# it carries the statically-linked Go program — but it also bakes a workspace file
# at /workspace/marker, which the `command` provider reads back.
#
# The marker does NOT discriminate where the provider ran: /workspace is the shared
# volume this image seeds, and every provider mounts it, so a provider running from
# its own image would read the marker just as well. What actually sets `command`
# apart is the program's ambient *toolchain* — binaries on PATH, which the mount
# (files, not a toolchain) cannot supply. Isolating that would mean baking a binary
# here and exec'ing it by name; the marker on its own only shows that the provider
# sees the program's workspace.
FROM alpine:3
RUN mkdir -p /workspace && echo "hello-from-the-program-workspace" > /workspace/marker
COPY program-linux /program
ENTRYPOINT ["/program"]
