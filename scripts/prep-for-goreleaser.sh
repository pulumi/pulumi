#!/bin/bash

# When run by goreleaser, FILTER_OS and DEST_DIR are blank, so files are installed in:
#
# * ./bin/darwin
# * ./bin/linux
# * ./bin/windows
#
# Allowing us to customize the archives for each.
#
# When run by GitHub Actions in tests, we set the first arg, FILTER_OS, so that
# we install only the current OS's binaries and in the shared "local path" dir, ./bin
FILTER_OS="$1"

COMMIT_TIME=$(git log -n1 --pretty='format:%cd' --date=format:'%Y%m%d%H%M')

install_file () {
    src="$1"
    shift

    for OS in "$@"; do # for each argument after the first:
        DESTDIR="bin"
        if [ -n "${FILTER_OS}" ]; then
            if [ "${FILTER_OS}" != "${OS}" ]; then
                continue
            fi
        else
            DESTDIR="bin/${OS}"
        fi
        mkdir -p "${DESTDIR}"
        dest=$(basename "${src}")
        cp "$src" "${DESTDIR}/${dest}"
        touch -t "${COMMIT_TIME}" "$dest"
    done
}

rm -rf ./bin
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
