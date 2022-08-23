#!/bin/sh

openapi-generator generate \
    --http-user-agent "pulumi-cli/1 ("`pulumictl get version -r ../../../../`", "`go env GOOS`")" \
    -c configuration.yaml \
    -i openapi.yaml \
    -g go \
    -o .
