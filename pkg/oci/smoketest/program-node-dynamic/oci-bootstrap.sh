#!/bin/sh
# Container entrypoint for an OCI-mode Node program — and, when asked, for the
# Node dynamic *provider*. The OCI engine reuses this same program image to serve
# a dynamic provider, because the SDK's dynamic-provider entrypoint is native to
# the image (it ships in @pulumi/pulumi). PULUMI_OCI_ROLE selects which role:
# unset -> run the program; "dynamic-provider" -> serve the provider. The base
# image owns this translation, so the engine stays language-agnostic.
set -e

# Dynamic-provider role: boot the SDK's dynamic-provider entrypoint instead of the
# program. Resolve it ambiently — exactly as the stock pulumi-resource-pulumi-nodejs
# shim does (require.resolve) — so we inherit the SDK's own module resolution. The
# entry historically requires an engine-address argv; that is vestigial (the engine
# dials the provider, not the reverse), but the arg must still be present, so pass
# PULUMI_ENGINE if set else a placeholder. The provider binds loopback and prints
# its port; the engine, sharing this netns, scrapes it and attaches.
if [ "$PULUMI_OCI_ROLE" = "dynamic-provider" ]; then
  SCRIPT="$(node -e "console.log(require.resolve('@pulumi/pulumi/cmd/dynamic-provider'))")"
  echo "oci-node-bootstrap: role=dynamic-provider -> exec node $SCRIPT" >&2
  exec node "$SCRIPT" "${PULUMI_ENGINE:-unused}"
fi

RUN_HARNESS=/workspace/node_modules/@pulumi/pulumi/cmd/run

# Build argv the way constructArguments does: skip empty optional flags.
set -- "$RUN_HARNESS"
[ -n "$PULUMI_MONITOR" ]      && set -- "$@" --monitor "$PULUMI_MONITOR"
[ -n "$PULUMI_ENGINE" ]       && set -- "$@" --engine "$PULUMI_ENGINE"
[ -n "$PULUMI_ORGANIZATION" ] && set -- "$@" --organization "$PULUMI_ORGANIZATION"
[ -n "$PULUMI_PROJECT" ]      && set -- "$@" --project "$PULUMI_PROJECT"
[ -n "$PULUMI_STACK" ]        && set -- "$@" --stack "$PULUMI_STACK"
set -- "$@" --pwd /workspace
[ "$PULUMI_DRY_RUN" = "true" ] && set -- "$@" --dry-run
set -- "$@" --parallel "${PULUMI_PARALLEL:-1}"
set -- "$@" .  # entry point: the program directory

echo "oci-node-bootstrap: monitor=$PULUMI_MONITOR engine=$PULUMI_ENGINE -> exec node $*" >&2
exec node "$@"
