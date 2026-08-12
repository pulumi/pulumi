import pulumi
import pulumi_asset_archive as asset_archive

ass = asset_archive.AssetResource("ass", value=pulumi.FileAsset("../test.txt"))
arc = asset_archive.ArchiveResource("arc", value=pulumi.FileArchive("../archive.tar"))
dir = asset_archive.ArchiveResource("dir", value=pulumi.FileArchive("../folder"))
assarc = asset_archive.ArchiveResource("assarc", value=pulumi.AssetArchive({
    "string": pulumi.StringAsset("file contents"),
    "file": pulumi.FileAsset("../test.txt"),
    "folder": pulumi.FileArchive("../folder"),
    "archive": pulumi.FileArchive("../archive.tar"),
}))
remoteass = asset_archive.AssetResource("remoteass", value=pulumi.RemoteAsset("https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/test.txt"))
remotearc = asset_archive.ArchiveResource("remotearc", value=pulumi.RemoteArchive("https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/archive.tar"))
pulumi.export("assetOutput", pulumi.FileAsset("../test.txt"))
pulumi.export("archiveOutput", pulumi.FileArchive("../archive.tar"))
pulumi.export("assetList", [
    pulumi.FileAsset("../test.txt"),
    pulumi.StringAsset("file contents"),
])
pulumi.export("archiveList", [
    pulumi.FileArchive("../archive.tar"),
    pulumi.FileArchive("../folder"),
])
pulumi.export("assetMap", {
    "file": pulumi.FileAsset("../test.txt"),
    "string": pulumi.StringAsset("file contents"),
})
pulumi.export("archiveMap", {
    "tar": pulumi.FileArchive("../archive.tar"),
    "folder": pulumi.FileArchive("../folder"),
})
