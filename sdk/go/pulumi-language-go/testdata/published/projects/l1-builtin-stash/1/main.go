package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		myStash, err := pulumi.NewStash(ctx, "myStash", &pulumi.StashArgs{
			Value: pulumi.Any("ignored"),
		})
		if err != nil {
			return err
		}
		ctx.Export("stashOutput", myStash.Value)
		passthroughStash, err := pulumi.NewStash(ctx, "passthroughStash", &pulumi.StashArgs{
			Value:       pulumi.Any("new"),
			Passthrough: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("passthroughOutput", passthroughStash.Value)
		return nil
	})
}
