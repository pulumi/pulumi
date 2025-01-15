package main

import (
	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		class, err := simple.NewResource(ctx, "class", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("class", class)
		export, err := simple.NewResource(ctx, "export", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("export", export)
		mod, err := simple.NewResource(ctx, "mod", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("mod", mod)
		_, err = simple.NewResource(ctx, "import", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		object, err := simple.NewResource(ctx, "object", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("object", object)
		self, err := simple.NewResource(ctx, "self", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("self", self)
		this, err := simple.NewResource(ctx, "this", &simple.ResourceArgs{
			Value: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		ctx.Export("this", this)
		return nil
	})
}
