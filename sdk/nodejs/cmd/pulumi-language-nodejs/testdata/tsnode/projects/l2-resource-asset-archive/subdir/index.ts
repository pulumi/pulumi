import * as pulumi from "@pulumi/pulumi";
import * as asset_archive from "@pulumi/asset-archive";

const ass = new asset_archive.AssetResource("ass", {value: new pulumi.asset.FileAsset("../test.txt")});
const arc = new asset_archive.ArchiveResource("arc", {value: new pulumi.asset.FileArchive("../archive.tar")});
const dir = new asset_archive.ArchiveResource("dir", {value: new pulumi.asset.FileArchive("../folder")});
