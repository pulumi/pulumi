import * as pulumi from "@pulumi/pulumi";
import * as asset_archive from "@pulumi/asset-archive";

const ass = new asset_archive.AssetResource("ass", {value: new pulumi.asset.FileAsset("../test.txt")});
const arc = new asset_archive.ArchiveResource("arc", {value: new pulumi.asset.FileArchive("../archive.tar")});
const dir = new asset_archive.ArchiveResource("dir", {value: new pulumi.asset.FileArchive("../folder")});
const assarc = new asset_archive.ArchiveResource("assarc", {value: new pulumi.asset.AssetArchive({
    string: new pulumi.asset.StringAsset("file contents"),
    file: new pulumi.asset.FileAsset("../test.txt"),
    folder: new pulumi.asset.FileArchive("../folder"),
    archive: new pulumi.asset.FileArchive("../archive.tar"),
})});
