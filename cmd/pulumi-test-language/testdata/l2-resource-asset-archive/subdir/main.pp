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