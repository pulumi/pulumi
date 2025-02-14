// Copyright 2025, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		custom1, err := NewCustom(ctx, "custom1", &CustomArgs{
			Value: pulumi.String("hello"),
		})
		if err != nil {
			return err
		}

		custom2, err := NewCustom(ctx, "custom2", &CustomArgs{
			Value: pulumi.String("world"),
		})
		if err != nil {
			return err
		}

		component1, err := NewComponent(ctx, "component1", &ComponentArgs{
			Resource:     custom1,
			ResourceList: []*Custom{custom1, custom2},
			ResourceMap: map[string]*Custom{
				"one": custom1,
				"two": custom2,
			},
		})
		if err != nil {
			return err
		}

		result, err := component1.Refs(ctx, &ComponentRefsArgs{
			Resource:     custom1,
			ResourceList: []CustomInput{custom1, custom2},
			ResourceMap: map[string]CustomInput{
				"one": custom1,
				"two": custom2,
			},
		})
		if err != nil {
			return err
		}

		ctx.Export("propertyDepsFromCall", result.Result())

		return nil
	})
}
