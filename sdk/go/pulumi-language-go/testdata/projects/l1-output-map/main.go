package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("empty", pulumi.Map{})
		ctx.Export("strings", pulumi.StringMap{
			"greeting": pulumi.String("Hello, world!"),
			"farewell": pulumi.String("Goodbye, world!"),
		})
		ctx.Export("numbers", pulumi.Float64Map{
			"1": pulumi.Float64(1),
			"2": pulumi.Float64(2),
		})
		ctx.Export("keys", pulumi.Float64Map{
			"my.key": pulumi.Float64(1),
			"my-key": pulumi.Float64(2),
			"my_key": pulumi.Float64(3),
			"MY_KEY": pulumi.Float64(4),
			"mykey":  pulumi.Float64(5),
			"MYKEY":  pulumi.Float64(6),
		})
		return nil
	})
}
