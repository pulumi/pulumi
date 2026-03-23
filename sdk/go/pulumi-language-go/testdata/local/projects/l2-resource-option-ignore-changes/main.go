package main

import (
	"example.com/pulumi-nestedobject/sdk/go/nestedobject"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := nestedobject.NewReceiver(ctx, "receiverIgnore", &nestedobject.ReceiverArgs{
			Details: nestedobject.DetailArray{
				&nestedobject.DetailArgs{
					Key:   pulumi.String("a"),
					Value: pulumi.String("b"),
				},
			},
		}, pulumi.IgnoreChanges([]string{
			"details[0].key",
		}))
		if err != nil {
			return err
		}
		_, err = nestedobject.NewMapContainer(ctx, "mapIgnore", &nestedobject.MapContainerArgs{
			Tags: pulumi.StringMap{
				"env": pulumi.String("prod"),
			},
		}, pulumi.IgnoreChanges([]string{
			"tags.env",
		}))
		if err != nil {
			return err
		}
		_, err = nestedobject.NewTarget(ctx, "noIgnore", &nestedobject.TargetArgs{
			Name: pulumi.String("nothing"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
