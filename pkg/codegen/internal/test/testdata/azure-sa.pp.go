package main

import (
	"github.com/pulumi/pulumi-azure/sdk/v3/go/azure/core"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		resourceGroupNameParam := cfg.Require("resourceGroupNameParam")
		resourceGroupVar, err := core.LookupResourceGroup(ctx, &core.LookupResourceGroupArgs{
			Name: resourceGroupNameParam,
		}, nil)
		if err != nil {
			return err
		}
		locationParam := resourceGroupVar.Location
		if param := cfg.Get("locationParam"); param != "" {
			locationParam = param
		}
		return nil
	})
}
