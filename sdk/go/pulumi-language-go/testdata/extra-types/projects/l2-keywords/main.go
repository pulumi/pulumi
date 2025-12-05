package main

import (
	"example.com/pulumi-keywords/sdk/go/v20/keywords"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		firstResource, err := keywords.NewSomeResource(ctx, "firstResource", &keywords.SomeResourceArgs{
			Builtins: pulumi.String("builtins"),
			Property: pulumi.String("property"),
		})
		if err != nil {
			return err
		}
		_, err = keywords.NewSomeResource(ctx, "secondResource", &keywords.SomeResourceArgs{
			Builtins: firstResource.Builtins,
			Property: firstResource.Property,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
