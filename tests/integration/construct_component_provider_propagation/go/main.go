//go:build !all
// +build !all

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		provider, err := NewRandomProvider(ctx, "explicit")
		if err != nil {
			return err
		}

		if _, err := NewComponent(ctx, "uses_default", nil); err != nil {
			return err
		}

		if _, err := NewComponent(ctx, "uses_provider", nil, pulumi.Provider(provider)); err != nil {
			return err
		}

		if _, err := NewComponent(ctx, "uses_providers", nil, pulumi.Providers(provider)); err != nil {
			return err
		}

		providerMap := map[string]pulumi.ProviderResource{
			"testprovider": provider,
		}
		if _, err := NewComponent(ctx, "uses_providers_map", nil, pulumi.ProviderMap(providerMap)); err != nil {
			return err
		}

		return nil
	})
}

type RandomProvider struct {
	pulumi.ProviderResourceState
}

func NewRandomProvider(ctx *pulumi.Context, name string) (*RandomProvider, error) {
	var provider RandomProvider
	err := ctx.RegisterResource("pulumi:providers:testprovider", "explicit", nil, &provider)
	return &provider, err
}

type Component struct {
	pulumi.ResourceState

	Result pulumi.StringOutput `pulumi:"result"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs, opts ...pulumi.ResourceOption) (*Component, error) {
	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, args, &resource, opts...)
	return &resource, err
}

type ComponentArgs struct {
	Result pulumi.StringInput `pulumi:"result"`
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*fooComponentArgs)(nil)).Elem()
}

type fooComponentArgs struct {
	Result pulumi.StringInput `pulumi:"result"`
}
