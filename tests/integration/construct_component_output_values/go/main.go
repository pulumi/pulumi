// Copyright 2016, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Dependency struct {
	pulumi.CustomResourceState
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		dep := &Dependency{}
		if err := ctx.RegisterResource("testprovider:index:Random", "dep", pulumi.Map{
			"length": pulumi.Int(1),
		}, dep); err != nil {
			return err
		}
		b := dep.URN().ApplyT(func(pulumi.URN) string {
			return "shh"
		}).(pulumi.StringOutput)

		if _, err := NewComponent(ctx, "component", &ComponentArgs{
			Foo: &FooArgs{
				Something: pulumi.String("hello"),
			},
			Bar: &BarArgs{
				Tags: pulumi.StringMap{
					"a": pulumi.String("world"),
					"b": b,
				},
			},
		}); err != nil {
			return err
		}
		return nil
	})
}
