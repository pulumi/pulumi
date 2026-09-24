#!/usr/bin/env bash
# Authenticode-signs a Windows binary with Azure Trusted Signing. Run by goreleaser as a post-build hook for every
# target, so it is a no-op unless the binary is a .exe and SIGN_WINDOWS_BINARIES=true.

set -euo pipefail

BINARY="$1"

if [[ "${BINARY}" != *.exe || "${SIGN_WINDOWS_BINARIES:-}" != "true" ]]; then
    exit 0
fi

: "${JSIGN_JAR:?JSIGN_JAR must be set}"
: "${AZURE_SIGNING_ACCESS_TOKEN:?AZURE_SIGNING_ACCESS_TOKEN must be set}"
: "${AZURE_SIGNING_ACCOUNT_ENDPOINT:?AZURE_SIGNING_ACCOUNT_ENDPOINT must be set}"
: "${AZURE_SIGNING_ACCOUNT_NAME:?AZURE_SIGNING_ACCOUNT_NAME must be set}"
: "${AZURE_SIGNING_CERT_PROFILE_NAME:?AZURE_SIGNING_CERT_PROFILE_NAME must be set}"

ENDPOINT_HOST="${AZURE_SIGNING_ACCOUNT_ENDPOINT#https://}"
ENDPOINT_HOST="${ENDPOINT_HOST%/}"

java -jar "${JSIGN_JAR}" \
    --storetype TRUSTEDSIGNING \
    --keystore "${ENDPOINT_HOST}" \
    --storepass "${AZURE_SIGNING_ACCESS_TOKEN}" \
    --alias "${AZURE_SIGNING_ACCOUNT_NAME}/${AZURE_SIGNING_CERT_PROFILE_NAME}" \
    "${BINARY}"
