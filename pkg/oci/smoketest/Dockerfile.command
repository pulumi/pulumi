# Program image for the workspace-coupled-provider smoke test. Like Dockerfile,
# it carries the statically-linked Go program — but it also bakes a workspace
# file at /workspace/marker. The `command` provider runs *from this image*
# (run-from-program-image), so it can read /workspace/marker even though that
# file exists in no provider image and in no copied volume: it is simply part of
# the program's filesystem, which the provider shares by being rooted in it.
FROM alpine:3
RUN mkdir -p /workspace && echo "hello-from-the-program-workspace" > /workspace/marker
COPY program-linux /program
ENTRYPOINT ["/program"]
