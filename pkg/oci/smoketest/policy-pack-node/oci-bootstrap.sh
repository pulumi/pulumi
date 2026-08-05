#!/bin/sh
# Container entrypoint for an OCI-mode policy pack. This image serves exactly one thing —
# the pack's analyzer — so the entrypoint is unconditional (no PULUMI_OCI_ROLE switch: that
# variable is for the multi-entrypoint program image, which serves either the program or a
# welded-in dynamic provider). We boot the SDK's run-policy-pack harness (native to
# @pulumi/pulumi installed here), resolving it ambiently the same way the stock
# pulumi-analyzer-policy shim does, so we inherit the SDK's own module resolution.
# run-policy-pack takes <engine-address> <program-dir>; the pack's code is at /policy. The
# pack's analyzer binds loopback by default, or the address the engine requests via
# PULUMI_PLUGIN_LISTEN_ADDRESS (address mode), and prints the bound port either way; the
# engine scrapes it and attaches over the shared netns or by container DNS. ts-node (a
# dependency of @pulumi/pulumi) compiles the TypeScript pack at run time — the toolchain
# that lives in this image and not the engine's.
set -e

SCRIPT="$(node -e "console.log(require.resolve('@pulumi/pulumi/cmd/run-policy-pack'))")"
echo "oci-policy-bootstrap: exec node $SCRIPT <engine> /policy" >&2
exec node "$SCRIPT" "${PULUMI_ENGINE:-unused}" /policy
