package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		myStash, err := pulumi.NewStash(ctx, "myStash", &pulumi.StashArgs{
			Input: pulumi.Any(map[string]interface{}{
				"key": []string{
					"value",
					"s",
				},
				"": false,
			}),
		})
		if err != nil {
			return err
		}
		ctx.Export("stashInput", myStash.Input)
		ctx.Export("stashOutput", myStash.Output)
		passthroughStash, err := pulumi.NewStash(ctx, "passthroughStash", &pulumi.StashArgs{
			Input:       pulumi.Any("old"),
			Passthrough: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("passthroughInput", passthroughStash.Input)
		ctx.Export("passthroughOutput", passthroughStash.Output)
		return nil
	})
}
