// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Provider struct {
	pulumi.ProviderResourceState

	Message pulumi.StringOutput `pulumi:"message"`
}

func NewProvider(ctx *pulumi.Context,
	name string, args *ProviderArgs, opts ...pulumi.ResourceOption,
) (*Provider, error) {
	if args == nil {
		args = &ProviderArgs{}
	}
	var resource Provider
	err := ctx.RegisterResource("pulumi:providers:testcomponent", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type providerArgs struct {
	Message string `pulumi:"message"`
}

type ProviderArgs struct {
	Message pulumi.StringInput
}

func (ProviderArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*providerArgs)(nil)).Elem()
}

// A remote component resource.
type Component struct {
	pulumi.ResourceState

	Message pulumi.StringOutput `pulumi:"message"`
}

// Creates a remote component resource.
func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, nil, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

// A local component resource.
type LocalComponent struct {
	pulumi.ResourceState

	Message pulumi.StringOutput
}

// Creates a regular local component resource, which creates a child remote component resource.
func NewLocalComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*LocalComponent, error) {
	var resource LocalComponent
	err := ctx.RegisterComponentResource("my:index:LocalComponent", name, &resource, opts...)
	if err != nil {
		return nil, err
	}

	component, err := NewComponent(ctx, name+"-mycomponent", pulumi.Parent(&resource))
	if err != nil {
		return nil, err
	}
	resource.Message = component.Message

	return &resource, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		provider, err := NewProvider(ctx, "myprovider", &ProviderArgs{
			Message: pulumi.String("hello world"),
		})
		if err != nil {
			return err
		}

		component, err := NewComponent(ctx, "mycomponent", pulumi.Provider(provider))
		if err != nil {
			return err
		}

		localComponent, err := NewLocalComponent(ctx, "mylocalcomponent", pulumi.Providers(provider))
		if err != nil {
			return err
		}

		ctx.Export("message", component.Message)
		ctx.Export("nestedMessage", localComponent.Message)

		return nil
	})
}
