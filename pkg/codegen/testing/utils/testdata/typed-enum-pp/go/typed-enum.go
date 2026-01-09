package main

import (
	storage "github.com/pulumi/pulumi-azure-native/sdk/go/azure/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		someString := "foobar"
		typeVar := "Block"
		staticwebsite, err := storage.NewStorageAccountStaticWebsite(ctx, "staticwebsite", &storage.StorageAccountStaticWebsiteArgs{
			ResourceGroupName: pulumi.String(someString),
			AccountName:       pulumi.String(someString),
		})
		if err != nil {
			return err
		}
		_, err = storage.NewBlob(ctx, "faviconpng", &storage.BlobArgs{
			ResourceGroupName: pulumi.String(someString),
			AccountName:       pulumi.String(someString),
			ContainerName:     pulumi.String(someString),
			Type:              storage.BlobTypeBlock,
		})
		if err != nil {
			return err
		}
		_, err = storage.NewBlob(ctx, "_404html", &storage.BlobArgs{
			ResourceGroupName: pulumi.String(someString),
			AccountName:       pulumi.String(someString),
			ContainerName:     pulumi.String(someString),
			Type:              staticwebsite.IndexDocument.ApplyT(func(x *string) storage.BlobType { return storage.BlobType(*x) }).(storage.BlobTypeOutput),
		})
		if err != nil {
			return err
		}
		_, err = storage.NewBlob(ctx, "another", &storage.BlobArgs{
			ResourceGroupName: pulumi.String(someString),
			AccountName:       pulumi.String(someString),
			ContainerName:     pulumi.String(someString),
			Type:              storage.BlobType(typeVar),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
