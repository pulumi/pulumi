#!/bin/sh
# Container entrypoint for an OCI-mode policy pack. The engine's container host runs
# this image with PULUMI_OCI_ROLE=policy-pack to serve the pack's analyzer. We boot
# the SDK's run-policy-pack harness (native to @pulumi/pulumi installed here),
# resolving it ambiently the same way the stock pulumi-analyzer-policy shim does, so
# we inherit the SDK's own module resolution. run-policy-pack takes
# <engine-address> <program-dir>; the pack's code is at /policy. The pack's analyzer
# binds loopback and prints its port; the engine, sharing this netns, scrapes it and
# attaches. ts-node (a dependency of @pulumi/pulumi) compiles the TypeScript pack at
# run time — the toolchain that lives in this image and not the engine's.
set -e

if [ "$PULUMI_OCI_ROLE" = "policy-pack" ]; then
  SCRIPT="$(node -e "console.log(require.resolve('@pulumi/pulumi/cmd/run-policy-pack'))")"
  echo "oci-policy-bootstrap: role=policy-pack -> exec node $SCRIPT <engine> /policy" >&2
  exec node "$SCRIPT" "${PULUMI_ENGINE:-unused}" /policy
fi

echo "oci-policy-bootstrap: PULUMI_OCI_ROLE!=policy-pack — this image only serves a policy pack" >&2
exit 1
