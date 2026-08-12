package main

import (
	"example.com/pulumi-config/sdk/go/v9/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		prov, err := config.NewProvider(ctx, "prov", &config.ProviderArgs{
			Name:              pulumi.String("my config"),
			PluginDownloadURL: pulumi.String("not the same as the pulumi resource option"),
		})
		if err != nil {
			return err
		}
		// Note this isn't _using_ the explicit provider, it's just grabbing a value from it.
		_, err = config.NewResource(ctx, "res", &config.ResourceArgs{
			Text: prov.Version,
		})
		if err != nil {
			return err
		}
		ctx.Export("pluginDownloadURL", prov.PluginDownloadURL)
		return nil
	})
}
