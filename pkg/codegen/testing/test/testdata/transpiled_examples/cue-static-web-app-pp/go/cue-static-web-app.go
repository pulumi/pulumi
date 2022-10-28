package main

import (
	resources "github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
	storage "github.com/pulumi/pulumi-azure-native/sdk/go/azure/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		rawkodeGroup, err := resources.NewResourceGroup(ctx, "rawkode-group", &resources.ResourceGroupArgs{
			Location: pulumi.String("WestUs"),
		})
		if err != nil {
			return err
		}
		rawkodeStorage, err := storage.NewStorageAccount(ctx, "rawkode-storage", &storage.StorageAccountArgs{
			ResourceGroupName: rawkodeGroup.Name,
			Kind:              pulumi.String("StorageV2"),
			Sku: &storage.SkuArgs{
				Name: pulumi.String("Standard_LRS"),
			},
		})
		if err != nil {
			return err
		}
		rawkodeWebsite, err := storage.NewStorageAccountStaticWebsite(ctx, "rawkode-website", &storage.StorageAccountStaticWebsiteArgs{
			ResourceGroupName: rawkodeGroup.Name,
			AccountName:       rawkodeStorage.Name,
			IndexDocument:     pulumi.String("index.html"),
			Error404Document:  pulumi.String("404.html"),
		})
		if err != nil {
			return err
		}
		_, err = storage.NewBlob(ctx, "rawkode-index.html", &storage.BlobArgs{
			ResourceGroupName: rawkodeGroup.Name,
			AccountName:       rawkodeStorage.Name,
			ContainerName:     rawkodeWebsite.ContainerName,
			ContentType:       pulumi.String("text/html"),
			Type:              storage.BlobTypeBlock,
			Source:            pulumi.NewFileAsset("./website/index.html"),
		})
		if err != nil {
			return err
		}
		stack72Group, err := resources.NewResourceGroup(ctx, "stack72-group", &resources.ResourceGroupArgs{
			Location: pulumi.String("WestUs"),
		})
		if err != nil {
			return err
		}
		stack72Storage, err := storage.NewStorageAccount(ctx, "stack72-storage", &storage.StorageAccountArgs{
			ResourceGroupName: stack72Group.Name,
			Kind:              pulumi.String("StorageV2"),
			Sku: &storage.SkuArgs{
				Name: pulumi.String("Standard_LRS"),
			},
		})
		if err != nil {
			return err
		}
		stack72Website, err := storage.NewStorageAccountStaticWebsite(ctx, "stack72-website", &storage.StorageAccountStaticWebsiteArgs{
			ResourceGroupName: stack72Group.Name,
			AccountName:       stack72Storage.Name,
			IndexDocument:     pulumi.String("index.html"),
			Error404Document:  pulumi.String("404.html"),
		})
		if err != nil {
			return err
		}
		_, err = storage.NewBlob(ctx, "stack72-index.html", &storage.BlobArgs{
			ResourceGroupName: stack72Group.Name,
			AccountName:       stack72Storage.Name,
			ContainerName:     stack72Website.ContainerName,
			ContentType:       pulumi.String("text/html"),
			Type:              storage.BlobTypeBlock,
			Source:            pulumi.NewFileAsset("./website/index.html"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
