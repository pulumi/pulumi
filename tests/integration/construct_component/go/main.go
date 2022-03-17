// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type componentArgs struct {
	Echo interface{} `pulumi:"echo"`
}

type ComponentArgs struct {
	Echo pulumi.Input
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

type Component struct {
	pulumi.ResourceState

	Echo    pulumi.AnyOutput    `pulumi:"echo"`
	ChildID pulumi.StringOutput `pulumi:"childId"`
	Secret  pulumi.StringOutput `pulumi:"secret"`
}

func NewComponent(
	ctx *pulumi.Context, name string, args *ComponentArgs, opts ...pulumi.ResourceOption) (*Component, error) {

	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}

	return &resource, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		componentA, err := NewComponent(ctx, "a", &ComponentArgs{Echo: pulumi.Int(42)})
		if err != nil {
			return err
		}
		_, err = NewComponent(ctx, "b", &ComponentArgs{Echo: componentA.Echo})
		if err != nil {
			return err
		}
		_, err = NewComponent(ctx, "C", &ComponentArgs{Echo: componentA.ChildID})
		if err != nil {
			return err
		}
		return nil
	})
}
