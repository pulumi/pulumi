import pulumi
import pulumi_asset_archive as asset_archive

ass = asset_archive.AssetResource("ass", value=pulumi.FileAsset("../test.txt"))
arc = asset_archive.ArchiveResource("arc", value=pulumi.FileArchive("../archive.tar"))
dir = asset_archive.ArchiveResource("dir", value=pulumi.FileArchive("../folder"))
