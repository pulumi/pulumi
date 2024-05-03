resource "ass" "asset-archive:index:AssetResource" {
    value = fileAsset("../test.txt")
}

resource "arc" "asset-archive:index:ArchiveResource" {
    value = fileArchive("../archive.tar")
}

resource "dir" "asset-archive:index:ArchiveResource" {
    value = fileArchive("../folder")
}