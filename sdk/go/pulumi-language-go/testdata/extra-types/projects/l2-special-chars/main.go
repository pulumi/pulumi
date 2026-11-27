package main

import (
	"example.com/pulumi-specialchars/sdk/go/v54/specialchars"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// The nested object type has property names containing characters that are not legal
		// identifier characters in most languages: "@timestamp" and "entity.name".
		first, err := specialchars.NewThing(ctx, "first", &specialchars.ThingArgs{
			Value: pulumi.String("hello"),
		})
		if err != nil {
			return err
		}
		ctx.Export("itemOutput", first.Item)
		ctx.Export("invoked", specialchars.GetItemOutput(ctx, specialchars.GetItemOutputArgs{
			Value: pulumi.String("world"),
		}, nil))
		return nil
	})
}
