package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		username, err := random.NewRandomPet(ctx, "username", &random.RandomPetArgs{})
		if err != nil {
			return err
		}

		ctx.Export("name", username.ID())
		return nil
	})
}
