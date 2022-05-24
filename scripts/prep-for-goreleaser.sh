#!/bin/bash

COMMIT_TIME=$(git log -n1 --pretty='format:%cd' --date=format:'%Y%m%d%H%M')

install_file () {
    src="$1"
    dest=$(basename "$src")
    cp "$src" "$dest"
    touch -t "$COMMIT_TIME" "$dest"
}

install_file sdk/nodejs/dist/pulumi-resource-pulumi-nodejs
install_file sdk/nodejs/dist/pulumi-resource-pulumi-nodejs.cmd
install_file sdk/python/dist/pulumi-resource-pulumi-python .
install_file sdk/python/dist/pulumi-resource-pulumi-python.cmd .
install_file sdk/python/dist/pulumi-python3-shim.cmd .
install_file sdk/python/dist/pulumi-python-shim.cmd .
install_file sdk/nodejs/dist/pulumi-analyzer-policy .
install_file sdk/nodejs/dist/pulumi-analyzer-policy.cmd .
install_file sdk/python/dist/pulumi-analyzer-policy-python .
install_file sdk/python/dist/pulumi-analyzer-policy-python.cmd .
install_file sdk/python/cmd/pulumi-language-python-exec .
