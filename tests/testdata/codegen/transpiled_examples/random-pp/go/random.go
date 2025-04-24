package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		randomPassword, err := random.NewRandomPassword(ctx, "randomPassword", &random.RandomPasswordArgs{
			Length:          pulumi.Int(16),
			Special:         pulumi.Bool(true),
			OverrideSpecial: pulumi.String("_%@"),
		})
		if err != nil {
			return err
		}
		ctx.Export("password", randomPassword.Result)
		return nil
	})
}
