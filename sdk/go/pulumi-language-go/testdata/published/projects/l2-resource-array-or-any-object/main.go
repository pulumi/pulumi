package main

import (
	"example.com/pulumi-array-or-any/sdk/go/arrayorany"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := arrayorany.NewObjectList(ctx, "objectList", &arrayorany.ObjectListArgs{
			Values: pulumi.Any{
				&arrayorany.ItemArgs{
					Name: pulumi.String("first"),
				},
				&arrayorany.ItemArgs{
					Name: pulumi.String("second"),
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = arrayorany.NewObjectList(ctx, "objectAny", &arrayorany.ObjectListArgs{
			Values: arrayorany.ItemArray{
				Any: true,
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
