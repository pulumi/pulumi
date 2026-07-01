#!/bin/sh
# OCI program entrypoint — EXPLICIT, not hidden in a base image. The OCI engine runs
# this image with generic PULUMI_* env vars (there is no CLI-arg shim from the engine, as
# there is for a host-spawned language plugin); this script translates them into what the
# language SDK's run harness expects and execs it. Keeping it in the project means you can
# use any base image you like and there is no hidden magic — edit it if you need to.
#
# PULUMI_OCI_ROLE selects the role this container plays:
#   unset             -> run the program (default)
#   "dynamic-provider" -> serve the SDK's dynamic-provider entrypoint (the OCI engine
#                         reuses this same image to run dynamic providers, since their
#                         implementation is native to @pulumi/pulumi installed here).
set -e

if [ "$PULUMI_OCI_ROLE" = "dynamic-provider" ]; then
  # Resolve the dynamic-provider entrypoint ambiently — exactly as the stock
  # pulumi-resource-pulumi-nodejs shim does — so we inherit the SDK's own module
  # resolution. The trailing engine-address argv is vestigial (the engine dials the
  # provider, not the reverse) but must be present.
  SCRIPT="$(node -e "console.log(require.resolve('@pulumi/pulumi/cmd/dynamic-provider'))")"
  echo "oci-node-bootstrap: role=dynamic-provider -> exec node $SCRIPT" >&2
  exec node "$SCRIPT" "${PULUMI_ENGINE:-unused}"
fi

# Program role: invoke the @pulumi/pulumi run harness with the arguments
# pulumi-language-nodejs's constructArguments would pass (skip empty optional flags).
# Config rides PULUMI_CONFIG / PULUMI_CONFIG_SECRET_KEYS in the environment.
RUN_HARNESS=/workspace/node_modules/@pulumi/pulumi/cmd/run

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
