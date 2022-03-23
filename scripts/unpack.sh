#!/usr/bin/env bash
#
# Unpacks release archives from `goreleaser-downloads` into
# `goreleaser-prebuilt`, restoring the filenames and structure that
# Goreleaser produces. Used in conjunction wtih `go-wrapper.sh` to
# avoid rebuilding tested binaries in the CI pipeline.

set -euo pipefail

unpack_archive()
{
    suffix="$1"
    archive="$2"

    if [ ! -f "$archive" ]; then
        return
    fi

    if [[ "$archive" == *.zip ]]; then
        unzip -d goreleaser-prebuilt "$archive"
        bin="goreleaser-prebuilt/pulumi/bin"
    else
        tar --directory goreleaser-prebuilt -xf "$archive"
        bin="goreleaser-prebuilt/pulumi"
    fi

    for exe in $(ls "$bin"/*)
    do
        name=$(basename "$exe")
        name=${name%.exe}
        dest="goreleaser-prebuilt/${name}-${suffix}"
        mkdir "$dest"
        cp "$exe" "$dest"
    done
    rm -rf goreleaser-prebuilt/pulumi
}

rm -rf goreleaser-prebuilt
mkdir goreleaser-prebuilt
unpack_archive unix_darwin_arm64     goreleaser-downloads/pulumi-*-darwin-arm64.tar.gz
unpack_archive unix_darwin_amd64     goreleaser-downloads/pulumi-*-darwin-x64.tar.gz
unpack_archive unix_linux_arm64      goreleaser-downloads/pulumi-*-linux-arm64.tar.gz
unpack_archive unix_linux_amd64      goreleaser-downloads/pulumi-*-linux-x64.tar.gz
unpack_archive windows_windows_amd64 goreleaser-downloads/pulumi-*-windows-x64.zip
