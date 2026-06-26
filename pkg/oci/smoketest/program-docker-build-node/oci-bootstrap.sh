#!/bin/sh
# Container entrypoint for an OCI-mode Node program. The OCI language host runs
# this image with generic PULUMI_* env vars (no CLI-arg shim from the engine);
# this script translates them into the exact arguments the @pulumi/pulumi run
# harness expects — the same ones pulumi-language-nodejs passes (see
# constructArguments in sdk/nodejs/cmd/pulumi-language-nodejs/main.go) — and execs
# it. The base image owns this translation, so the engine stays language-agnostic.
# Config rides PULUMI_CONFIG / PULUMI_CONFIG_SECRET_KEYS in the environment.
set -e

RUN_HARNESS=/app/node_modules/@pulumi/pulumi/cmd/run

# Build argv the way constructArguments does: skip empty optional flags.
set -- "$RUN_HARNESS"
[ -n "$PULUMI_MONITOR" ]      && set -- "$@" --monitor "$PULUMI_MONITOR"
[ -n "$PULUMI_ENGINE" ]       && set -- "$@" --engine "$PULUMI_ENGINE"
[ -n "$PULUMI_ORGANIZATION" ] && set -- "$@" --organization "$PULUMI_ORGANIZATION"
[ -n "$PULUMI_PROJECT" ]      && set -- "$@" --project "$PULUMI_PROJECT"
[ -n "$PULUMI_STACK" ]        && set -- "$@" --stack "$PULUMI_STACK"
set -- "$@" --pwd /app
[ "$PULUMI_DRY_RUN" = "true" ] && set -- "$@" --dry-run
set -- "$@" --parallel "${PULUMI_PARALLEL:-1}"
set -- "$@" .  # entry point: the program directory

echo "oci-node-bootstrap: monitor=$PULUMI_MONITOR engine=$PULUMI_ENGINE -> exec node $*" >&2
exec node "$@"
