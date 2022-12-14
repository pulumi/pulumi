//go:build !all
// +build !all

package main

import (
	"errors"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		r, err := NewRandom(ctx, "default", &RandomArgs{
			Length: pulumi.Int(10),
		}, pulumi.PluginDownloadURL("get.example.test"))
		if err != nil {
			return err
		}

		provider, err := NewProvider(ctx, "explicit",
			pulumi.PluginDownloadURL("get.pulumi.test/providers"))
		e, err := NewRandom(ctx, "explicit", &RandomArgs{
			Length: pulumi.Int(8),
		}, pulumi.Provider(provider))
		ctx.Export("default provider", r.Result)
		ctx.Export("explicit provider", e.Result)
		return nil
	})
}

type Random struct {
	pulumi.CustomResourceState

	Length pulumi.IntOutput    `pulumi:"length"`
	Result pulumi.StringOutput `pulumi:"result"`
}

func NewProvider(ctx *pulumi.Context, name string,
	opts ...pulumi.ResourceOption) (pulumi.ProviderResource, error) {
	provider := Provider{}
	err := ctx.RegisterResource("pulumi:providers:testprovider",
		"provider", nil, &provider, opts...)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

type Provider struct {
	pulumi.ProviderResourceState
}

func NewRandom(ctx *pulumi.Context,
	name string, args *RandomArgs, opts ...pulumi.ResourceOption) (*Random, error) {
	if args == nil || args.Length == nil {
		return nil, errors.New("missing required argument 'Length'")
	}
	if args == nil {
		args = &RandomArgs{}
	}
	var resource Random
	err := ctx.RegisterResource("testprovider:index:Random", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type randomArgs struct {
	Length int `pulumi:"length"`
}

type RandomArgs struct {
	Length pulumi.IntInput
}

func (RandomArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*randomArgs)(nil)).Elem()
}
