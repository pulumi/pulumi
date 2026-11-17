#!/usr/bin/env bash

set -euo pipefail

ROOT_CA="$(dirname "$0")/microsoft-identity-verification-root-ca-2020.pem"
# osslsigncode 2.10 switched from slash-separated subjects to RFC 2253 format.
# Accept both Ubuntu 24.04's osslsigncode 2.8 and Ubuntu 26.04's osslsigncode 2.13
# while ubuntu-latest runners transition between these images.
EXPECTED_SIGNER="CN=PULUMI CORPORATION,O=PULUMI CORPORATION"
EXPECTED_SIGNER_LEGACY="/O=PULUMI CORPORATION/CN=PULUMI CORPORATION"

if [[ $# -eq 0 ]]; then
    echo "no binaries to check" >&2
    exit 1
fi

failed=0
for binary in "$@"; do
    if ! output="$(osslsigncode verify -CAfile "${ROOT_CA}" -TSA-CAfile "${ROOT_CA}" -in "${binary}" 2>&1)"; then
        echo "invalid signature: ${binary}"
        echo "${output}" | tail -5
        failed=1
    elif ! echo "${output}" | grep -m1 -A1 'Signer #0:' |
        grep -Eq -e "^[[:space:]]*Subject: ${EXPECTED_SIGNER}(,|$)" -e "${EXPECTED_SIGNER_LEGACY}$"; then
        echo "unexpected signer: ${binary}"
        echo "${output}" | grep -m1 -A1 'Signer #0:' | head -2
        failed=1
    else
        echo "signed: ${binary}"
    fi
done

exit "${failed}"
