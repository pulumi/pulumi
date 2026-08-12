package main

import (
	kebabmodule "example.com/pulumi-kebab-names/sdk/go/v52/kebabnames/kebab-module"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// The package name, module name and property names are kebab-case. Resource and object type names
		// cannot be kebab-case yet: the metaschema forbids hyphens in the member segment of a token.
		first, err := kebabmodule.NewSomeResource(ctx, "first", &kebabmodule.SomeResourceArgs{
			TheInput: pulumi.Bool(true),
			Nested: &kebabmodule.NestedInputArgs{
				NestedValue: pulumi.String("nested"),
			},
		})
		if err != nil {
			return err
		}
		_, err = kebabmodule.NewAnotherResource(ctx, "second", &kebabmodule.AnotherResourceArgs{
			TheInput: first.TheOutput.NestedOutput(),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
