// Copyright 2016-2022, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

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

func NewSecondComponent(
	ctx *pulumi.Context, name string, args *ComponentArgs, opts ...pulumi.ResourceOption) (*Component, error) {

	var resource Component
	err := ctx.RegisterRemoteComponentResource("secondtestcomponent:index:Component", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}

	return &resource, nil
}

func NewComponentComponent(
	ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {

	var resource Component
	err := ctx.RegisterRemoteComponentResource("secondtestcomponent:index:ComponentComponent", name, pulumi.Map{}, &resource, opts...)
	if err != nil {
		return nil, err
	}

	return &resource, nil
}

type provider struct {
	pulumi.ProviderResourceState
	expectResourceArg pulumi.Bool
}

type LocalComponent struct{ pulumi.ResourceState }

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

		provider := &provider{}
		err = ctx.RegisterResource("pulumi:providers:testcomponent", "provider", pulumi.Map{
			"expectResourceArg": pulumi.Bool(true),
		}, provider)
		if err != nil {
			return err
		}
		localComponent := &LocalComponent{}
		err = ctx.RegisterComponentResource("pkg:index:LocalComponent", "localComponent", localComponent, pulumi.Providers(provider))
		if err != nil {
			return err
		}
		parentProvider := pulumi.Parent(localComponent)
		_, err = NewComponent(ctx, "checkProvider1",
			&ComponentArgs{Echo: pulumi.String("checkExpected")}, parentProvider)
		if err != nil {
			return err
		}
		_, err = NewSecondComponent(ctx, "checkProvider2",
			&ComponentArgs{Echo: pulumi.String("checkExpected")}, parentProvider)
		if err != nil {
			return err
		}
		_, err = NewComponentComponent(ctx, "checkProvider12", parentProvider)
		if err != nil {
			return err
		}
		return nil
	})
}
