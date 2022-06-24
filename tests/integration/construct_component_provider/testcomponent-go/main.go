// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

type Provider struct {
	pulumi.ProviderResourceState

	Message pulumi.StringOutput `pulumi:"message"`
}

type Component struct {
	pulumi.ResourceState

	Message pulumi.StringOutput `pulumi:"message"`
}

func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Test that we're indeed getting back an instance of `Provider` with its state.
	provider := component.GetProvider("testcomponent::").(*Provider)
	component.Message = provider.Message

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"message": component.Message,
	}); err != nil {
		return nil, err
	}

	return component, nil
}

const providerName = "testcomponent"
const version = "0.0.1"

type pkg struct {
	version semver.Version
}

func (p *pkg) Version() semver.Version {
	return p.version
}

func (p *pkg) ConstructProvider(ctx *pulumi.Context, name, typ, urn string) (pulumi.ProviderResource, error) {
	if typ != "pulumi:providers:testcomponent" {
		return nil, fmt.Errorf("unknown provider type: %s", typ)
	}

	r := &Provider{}
	err := ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))
	return r, err
}

func main() {
	pulumi.RegisterResourcePackage(providerName, &pkg{semver.MustParse(version)})

	if err := provider.ComponentMain(providerName, version, nil, func(ctx *pulumi.Context, typ, name string,
		inputs pulumiprovider.ConstructInputs, options pulumi.ResourceOption) (*pulumiprovider.ConstructResult, error) {

		if typ != "testcomponent:index:Component" {
			return nil, fmt.Errorf("unknown resource type %s", typ)
		}

		component, err := NewComponent(ctx, name, options)
		if err != nil {
			return nil, fmt.Errorf("creating component: %w", err)
		}

		return pulumiprovider.NewConstructResult(component)
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
