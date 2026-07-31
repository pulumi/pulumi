#!/bin/sh
# Container entrypoint for an OCI-mode Python policy pack. The engine's container
# host runs this image with PULUMI_OCI_ROLE=policy-pack to serve the pack's
# analyzer. Unlike Node (whose pack is loaded by the run-policy-pack harness), a
# Python policy pack is its own server: instantiating PolicyPack in __main__.py
# binds the analyzer port and blocks serving. The SDK takes no argv — the serve
# address comes from PULUMI_PLUGIN_LISTEN_ADDRESS (address mode) or defaults to a
# loopback ephemeral port (netns mode), and the bound port is printed to stdout
# either way for the engine to scrape. -u keeps that handshake unbuffered.
set -e

if [ "$PULUMI_OCI_ROLE" = "policy-pack" ]; then
  echo "oci-policy-bootstrap: role=policy-pack -> exec python -u /policy/__main__.py" >&2
  exec python -u /policy/__main__.py
fi

echo "oci-policy-bootstrap: PULUMI_OCI_ROLE!=policy-pack — this image only serves a policy pack" >&2
exit 1
