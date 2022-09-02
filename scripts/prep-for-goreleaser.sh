#!/bin/bash

# Populates ./bin directories for use by goreleaser.
rm -rf ./bin

COMMIT_TIME=$(git log -n1 --pretty='format:%cd' --date=format:'%Y%m%d%H%M')

install_file () {
    src="$1"
    shift

    for OS in "$@"; do # for each argument after the first:
        DESTDIR="bin/${OS}"
        mkdir -p "${DESTDIR}"
        dest=$(basename "${src}")
        cp "$src" "${DESTDIR}/${dest}"
        touch -t "${COMMIT_TIME}" "$dest"
    done
}

install_file sdk/nodejs/dist/pulumi-analyzer-policy                         linux   darwin
install_file sdk/nodejs/dist/pulumi-analyzer-policy.cmd                     windows

install_file sdk/nodejs/dist/pulumi-resource-pulumi-nodejs                  linux   darwin
install_file sdk/nodejs/dist/pulumi-resource-pulumi-nodejs.cmd              windows

install_file sdk/python/dist/pulumi-analyzer-policy-python                  linux   darwin
install_file sdk/python/dist/pulumi-analyzer-policy-python.cmd              windows

install_file sdk/python/dist/pulumi-resource-pulumi-python                  linux   darwin
install_file sdk/python/dist/pulumi-resource-pulumi-python.cmd              windows

install_file sdk/python/dist/pulumi-python-shim.cmd                         windows
install_file sdk/python/dist/pulumi-python3-shim.cmd                        windows

install_file sdk/python/cmd/pulumi-language-python-exec          linux darwin windows

# Get pulumi-watch binaries
./scripts/get-pulumi-watch.sh
./scripts/get-language-providers.sh
