// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type MyComponent struct {
	pulumi.ResourceState
}

func NewMyComponent(ctx *pulumi.Context, name string) (*MyComponent, error) {
	component := &MyComponent{}

	err := ctx.RegisterComponentResource("test:index:MyComponent", name, component)
	if err != nil {
		return nil, err
	}

	return component, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		err := ctx.Log.Debug("A debug message", nil)
		if err != nil {
			return err
		}

		_, err = NewMyComponent(ctx, "mycomponent")
		return err
	})
}
