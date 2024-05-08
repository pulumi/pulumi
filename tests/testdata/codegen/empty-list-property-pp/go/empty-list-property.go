package main

import (
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := storage.NewStorageAccount(ctx, "storageAccounts", &storage.StorageAccountArgs{
			AccountName:       pulumi.String("sto4445"),
			Kind:              pulumi.String(storage.KindBlockBlobStorage),
			Location:          pulumi.String("eastus"),
			ResourceGroupName: pulumi.String("res9101"),
			Sku: &storage.SkuArgs{
				Name: pulumi.String(storage.SkuName_Premium_LRS),
			},
			NetworkRuleSet: &storage.NetworkRuleSetArgs{
				DefaultAction: storage.DefaultActionAllow,
				IpRules:       storage.IPRuleArray{},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
