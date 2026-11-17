package main

import (
	"example.com/pulumi-exists/sdk/go/v60/exists"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		res, err := exists.NewResource(ctx, "res", &exists.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		existsResult := ctx.ExistsResource("exists:index:Resource", res.ID(), nil, "")
		existsWithState := ctx.ExistsResource("exists:index:Resource", res.ID(), pulumi.Map{
			"value": pulumi.Any(true),
		}, "")
		missingWithState := ctx.ExistsResource("exists:index:Resource", res.ID(), pulumi.Map{
			"value": pulumi.Any(false),
		}, "")
		ctx.Export("existsResult", existsResult)
		ctx.Export("existsWithState", existsWithState)
		ctx.Export("missingWithState", missingWithState)
		return nil
	})
}
