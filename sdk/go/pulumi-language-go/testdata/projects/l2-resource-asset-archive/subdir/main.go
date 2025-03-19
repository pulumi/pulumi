package main

import (
	"example.com/pulumi-asset-archive/sdk/go/v5/assetarchive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := assetarchive.NewAssetResource(ctx, "ass", &assetarchive.AssetResourceArgs{
			Value: pulumi.NewFileAsset("../test.txt"),
		})
		if err != nil {
			return err
		}
		_, err = assetarchive.NewArchiveResource(ctx, "arc", &assetarchive.ArchiveResourceArgs{
			Value: pulumi.NewFileArchive("../archive.tar"),
		})
		if err != nil {
			return err
		}
		_, err = assetarchive.NewArchiveResource(ctx, "dir", &assetarchive.ArchiveResourceArgs{
			Value: pulumi.NewFileArchive("../folder"),
		})
		if err != nil {
			return err
		}
		_, err = assetarchive.NewArchiveResource(ctx, "assarc", &assetarchive.ArchiveResourceArgs{
			Value: pulumi.NewAssetArchive(map[string]interface{}{
				"string":  pulumi.NewStringAsset("file contents"),
				"file":    pulumi.NewFileAsset("../test.txt"),
				"folder":  pulumi.NewFileArchive("../folder"),
				"archive": pulumi.NewFileArchive("../archive.tar"),
			}),
		})
		if err != nil {
			return err
		}
		_, err = assetarchive.NewAssetResource(ctx, "remoteass", &assetarchive.AssetResourceArgs{
			Value: pulumi.NewRemoteAsset("https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/test.txt"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
