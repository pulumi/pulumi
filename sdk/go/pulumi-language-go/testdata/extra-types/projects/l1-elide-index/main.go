package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Test that "pkg:typ" type tokens are accepted in PCL and are correctly expanded out. We also have an L2 test around
		// this but it's worth checking with the pulumi schema as it would be too easy for codegen to special case it differently.
		myStash, err := pulumi.NewStash(ctx, "myStash", &pulumi.StashArgs{
			Input: pulumi.Any("test"),
		})
		if err != nil {
			return err
		}
		ctx.Export("stashInput", myStash.Input)
		ctx.Export("stashOutput", myStash.Output)
		return nil
	})
}
