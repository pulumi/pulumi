// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		if _, err := NewComponent(ctx, "component", &ComponentArgs{
			Foo: &FooArgs{
				Something: pulumi.String("hello"),
			},
			Bar: &BarArgs{
				Tags: pulumi.StringMap{
					"a": pulumi.String("world"),
					"b": pulumi.ToSecret("shh").(pulumi.StringOutput),
				},
			},
		}); err != nil {
			return err
		}
		return nil
	})
}
