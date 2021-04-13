// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Component struct {
	pulumi.ResourceState
}

func NewComponent(ctx *pulumi.Context, name string, args ComponentArgs,
	opts ...pulumi.ResourceOption) (*Component, error) {
	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, &args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type componentArgs struct {
	Children *int `pulumi:"children"`
}

type ComponentArgs struct {
	Children *int
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		children := 5
		if _, err := NewComponent(ctx, "component", ComponentArgs{Children: &children}); err != nil {
			return err
		}
		return nil
	})
}
