package main

import (
	"example.com/pulumi-inheritabstract/sdk/go/inheritabstract"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		child, err := inheritabstract.NewConcreteChild(ctx, "child", &inheritabstract.ConcreteChildArgs{
			Seed:  pulumi.String("s"),
			Extra: pulumi.String("e"),
		})
		if err != nil {
			return err
		}
		ctx.Export("abstractOutput", child.AbstractOutput)
		ctx.Export("concreteOutput", child.ConcreteOutput)
		return nil
	})
}
