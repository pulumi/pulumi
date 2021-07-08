// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type componentArgs struct {
	First  string `pulumi:"first"`
	Second string `pulumi:"second"`
}

type ComponentArgs struct {
	First  pulumi.StringInput
	Second pulumi.StringInput
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

type Component struct {
	pulumi.ResourceState
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

func (c *Component) GetMessage(ctx *pulumi.Context, args *ComponentGetMessageArgs) (ComponentGetMessageResultOutput, error) {
	out, err := ctx.Call("testcomponent:index:Component/getMessage", args, ComponentGetMessageResultOutput{}, c)
	if err != nil {
		return ComponentGetMessageResultOutput{}, err
	}
	return out.(ComponentGetMessageResultOutput), nil
}

type componentGetMessageArgs struct {
	Name string `pulumi:"name"`
}

type ComponentGetMessageArgs struct {
	Name pulumi.StringInput
}

func (ComponentGetMessageArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentGetMessageArgs)(nil)).Elem()
}

type ComponentGetMessageResult struct {
	Message string `pulumi:"message"`
}

type ComponentGetMessageResultOutput struct{ *pulumi.OutputState }

func (ComponentGetMessageResultOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ComponentGetMessageResult)(nil)).Elem()
}

func (o ComponentGetMessageResultOutput) Message() pulumi.StringOutput {
	return o.ApplyT(func(v ComponentGetMessageResult) string { return v.Message }).(pulumi.StringOutput)
}

func init() {
	pulumi.RegisterOutputType(ComponentGetMessageResultOutput{})
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		component, err := NewComponent(ctx, "component", &ComponentArgs{
			First:  pulumi.String("Hello"),
			Second: pulumi.String("World"),
		})
		if err != nil {
			return err
		}
		result, err := component.GetMessage(ctx, &ComponentGetMessageArgs{
			Name: pulumi.String("Alice"),
		})
		if err != nil {
			return err
		}
		ctx.Export("message", result.Message())
		return nil
	})
}
