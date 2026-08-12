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

resource "remotearc" "asset-archive:index:ArchiveResource" {
    value = remoteArchive("https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/archive.tar")
}

// Plain (non-nested) asset/archive outputs must round-trip through stack
// outputs in every language.
output "assetOutput" {
    value = fileAsset("../test.txt")
}

output "archiveOutput" {
    value = fileArchive("../archive.tar")
}

// Regression test for pulumi/pulumi#16384: array and map of assets/archives
// must compose properly through Go program generation.
output "assetList" {
    value = [fileAsset("../test.txt"), stringAsset("file contents")]
}

output "archiveList" {
    value = [fileArchive("../archive.tar"), fileArchive("../folder")]
}

output "assetMap" {
    value = {
        "file":   fileAsset("../test.txt"),
        "string": stringAsset("file contents"),
    }
}

output "archiveMap" {
    value = {
        "tar":    fileArchive("../archive.tar"),
        "folder": fileArchive("../folder"),
    }
}
