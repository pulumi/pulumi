#!/bin/sh
# Container entrypoint for an OCI-mode Python program — and, when asked, the Python
# dynamic *provider*. The OCI engine reuses this same program image to serve the
# dynamic provider, whose entrypoint (python -m pulumi.dynamic) is native to the
# pulumi SDK installed here. PULUMI_OCI_ROLE selects the role: unset -> run the
# program; "dynamic-provider" -> serve the provider. The base image owns this
# translation, so the engine stays language-agnostic.
set -e

# Dynamic-provider role: boot the SDK's dynamic-provider entrypoint instead of the
# program. The Python entry (unlike Node) does not validate the engine arg; pass
# PULUMI_ENGINE through anyway for parity. The provider binds loopback and prints
# its port; the engine, sharing this netns, scrapes it and attaches.
if [ "$PULUMI_OCI_ROLE" = "dynamic-provider" ]; then
  echo "oci-python-bootstrap: role=dynamic-provider -> exec python -m pulumi.dynamic" >&2
  exec python3 -u -m pulumi.dynamic "${PULUMI_ENGINE:-unused}"
fi

# Program role: invoke the SDK's program-exec shim (copied into this image — it
# ships with the CLI, not the pip package), translating PULUMI_* env into the args
# pulumi-language-python's constructArguments passes. Config rides PULUMI_CONFIG /
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
