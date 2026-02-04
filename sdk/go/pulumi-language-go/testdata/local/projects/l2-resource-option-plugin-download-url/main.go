package main

import (
	"example.com/pulumi-simple/sdk/go/v27/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := simple.NewResource(ctx, "withDefaultURL", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "withExplicitDefaultURL", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "withCustomURL1", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.PluginDownloadURL("https://custom.pulumi.test/provider1"))
		if err != nil {
			return err
		}
		_, err = simple.NewResource(ctx, "withCustomURL2", &simple.ResourceArgs{
			Value: pulumi.Bool(false),
		}, pulumi.PluginDownloadURL("https://custom.pulumi.test/provider2"))
		if err != nil {
			return err
		}
		return nil
	})
}
