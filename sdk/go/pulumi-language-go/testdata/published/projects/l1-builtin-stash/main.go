package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		myStash, err := pulumi.NewStash(ctx, "myStash", pulumi.Any(map[string]interface{}{
			"key": []string{
				"value",
				"s",
			},
			"": false,
		}))
		if err != nil {
			return err
		}
		ctx.Export("stashOutput", myStash.Value)
		return nil
	})
}
