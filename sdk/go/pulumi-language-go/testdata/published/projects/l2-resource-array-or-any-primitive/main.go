package main

import (
	"example.com/pulumi-array-or-any/sdk/go/arrayorany"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := arrayorany.NewStringList(ctx, "stringList", &arrayorany.StringListArgs{
			Values: pulumi.Any{
				pulumi.String("a"),
				pulumi.String("b"),
				pulumi.String("c"),
			},
		})
		if err != nil {
			return err
		}
		_, err = arrayorany.NewStringList(ctx, "stringAny", &arrayorany.StringListArgs{
			Values: pulumi.StringArray{
				Any: pulumi.String(true),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
