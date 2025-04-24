// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type TestProvider struct {
	pulumi.ProviderResourceState
}

func NewTestProvider(ctx *pulumi.Context, name string) (*TestProvider, error) {
	var resource TestProvider
	err := ctx.RegisterResource("pulumi:providers:testprovider", name, nil, &resource)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

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
	ctx *pulumi.Context, name string, args *ComponentArgs, opts ...pulumi.ResourceOption,
) (*Component, error) {
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

func (*Component) ElementType() reflect.Type {
	return reflect.TypeOf((*Component)(nil))
}

func init() {
	pulumi.RegisterOutputType(ComponentGetMessageResultOutput{})
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		testProvider, err := NewTestProvider(ctx, "testProvider")
		if err != nil {
			return err
		}

		component1, err := NewComponent(ctx, "component1", &ComponentArgs{
			First:  pulumi.String("Hello"),
			Second: pulumi.String("World"),
		}, pulumi.Provider(testProvider))
		if err != nil {
			return err
		}
		result1, err := component1.GetMessage(ctx, &ComponentGetMessageArgs{
			Name: pulumi.String("Alice"),
		})
		if err != nil {
			return err
		}

		component2, err := NewComponent(ctx, "component2", &ComponentArgs{
			First:  pulumi.String("Hi"),
			Second: pulumi.String("There"),
		}, pulumi.Providers(testProvider))
		if err != nil {
			return err
		}
		result2, err := component2.GetMessage(ctx, &ComponentGetMessageArgs{
			Name: pulumi.String("Bob"),
		})
		if err != nil {
			return err
		}

		ctx.Export("message1", result1.Message())
		ctx.Export("message2", result2.Message())

		return nil
	})
}
