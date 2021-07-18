package main

import (
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")
		length := conf.RequireInt("length")

		password, err := random.NewRandomPassword(ctx, "password", &random.RandomPasswordArgs{
			Length:          pulumi.Int(length),
			Special:         pulumi.Bool(true),
			OverrideSpecial: pulumi.String(fmt.Sprintf("%v%v%v", "_", "%", "@")),
		})
		if err != nil {
			return err
		}
		ctx.Export("password", password.Result)
		return nil
	})
}
