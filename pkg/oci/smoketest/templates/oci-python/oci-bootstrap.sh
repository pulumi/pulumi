#!/bin/sh
# OCI program entrypoint — EXPLICIT, not hidden in a base image. The OCI engine runs this
# image with generic PULUMI_* env vars; this script translates them into what the Python
# SDK's run harness expects and execs it. Keeping it in the project means you can use any
# Python base image and there is no hidden magic — edit it if you need to.
#
# PULUMI_OCI_ROLE selects the role this container plays:
#   unset             -> run the program (default)
#   "dynamic-provider" -> serve the SDK's dynamic-provider entrypoint (python -m
#                         pulumi.dynamic), which is native to the pulumi SDK installed here.
set -e

if [ "$PULUMI_OCI_ROLE" = "dynamic-provider" ]; then
  echo "oci-python-bootstrap: role=dynamic-provider -> exec python -m pulumi.dynamic" >&2
  exec python3 -u -m pulumi.dynamic "${PULUMI_ENGINE:-unused}"
fi

# Program role: invoke the SDK's program-exec shim (pulumi-language-python-exec, vendored
# into this image — see the Dockerfile note), translating PULUMI_* env into the arguments
# pulumi-language-python's constructArguments would pass. Config rides PULUMI_CONFIG /
# PULUMI_CONFIG_SECRET_KEYS in the environment.
set --
[ -n "$PULUMI_MONITOR" ]      && set -- "$@" --monitor "$PULUMI_MONITOR"
[ -n "$PULUMI_ENGINE" ]       && set -- "$@" --engine "$PULUMI_ENGINE"
[ -n "$PULUMI_PROJECT" ]      && set -- "$@" --project "$PULUMI_PROJECT"
[ -n "$PULUMI_STACK" ]        && set -- "$@" --stack "$PULUMI_STACK"
[ -n "$PULUMI_ORGANIZATION" ] && set -- "$@" --organization "$PULUMI_ORGANIZATION"
set -- "$@" --pwd /app --dry_run "${PULUMI_DRY_RUN:-false}" --parallel "${PULUMI_PARALLEL:-1}" ./__main__.py

echo "oci-python-bootstrap: monitor=$PULUMI_MONITOR engine=$PULUMI_ENGINE -> exec pulumi-language-python-exec $*" >&2
exec python3 /app/pulumi-language-python-exec "$@"
