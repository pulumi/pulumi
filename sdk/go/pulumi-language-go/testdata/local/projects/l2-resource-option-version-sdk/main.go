package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Check that withV2 is generated against the v2 SDK and not against the V26 SDK,
		// and that the version resource option is elided.
		_, err := simple.NewResource(ctx, "withV2", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		}, pulumi.Version("2.0.0"))
		if err != nil {
			return err
		}
		return nil
	})
}
