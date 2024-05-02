resource "ass" "asset-archive:index:AssetResource" {
    value = fileAsset("../test.txt")
}

resource "arc" "asset-archive:index:ArchiveResource" {
    value = fileArchive("../archive.tar")
}