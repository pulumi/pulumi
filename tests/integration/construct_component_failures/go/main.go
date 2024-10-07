// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewComponent(ctx, "component", &ComponentArgs{
			Foo: pulumi.String("bar"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
