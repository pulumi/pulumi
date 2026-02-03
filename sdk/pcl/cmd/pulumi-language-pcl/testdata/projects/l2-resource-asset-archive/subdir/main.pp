resource "ass" "asset-archive:index:AssetResource" {
    value = fileAsset("../test.txt")
}

resource "arc" "asset-archive:index:ArchiveResource" {
    value = fileArchive("../archive.tar")
}

resource "dir" "asset-archive:index:ArchiveResource" {
    value = fileArchive("../folder")
}

resource "assarc" "asset-archive:index:ArchiveResource" {
    value = assetArchive({
        "string": stringAsset("file contents"),
        "file": fileAsset("../test.txt"),
        "folder": fileArchive("../folder"),
        "archive": fileArchive("../archive.tar"),
    })
}

resource "remoteass" "asset-archive:index:AssetResource" {
    value = remoteAsset("https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/test.txt")
}
