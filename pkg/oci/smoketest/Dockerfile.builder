# A discriminating builder image for run-pod-builder-container.sh.
#
# It is the docker CLI plus a marker file that exists ONLY here — standing in for a
# build toolchain (nix/bazel/buildpacks/…) that the engine image does not carry. The
# build command guards on this marker, so if the build were (mis)routed to the
# in-process/engine path the marker would be absent and the build would fail loudly.
# A passing run therefore proves the build executed in this builder container.
FROM docker:cli
RUN touch /only-in-builder
