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
