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

type Component struct {
	pulumi.ResourceState
}

func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	component := &Component{}
	if err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...); err != nil {
		return nil, err
	}
	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{}); err != nil {
		return nil, err
	}
	return component, nil
}

type ComponentCreateRandomArgs struct {
	Length pulumi.IntInput `pulumi:"length"`
}

type ComponentCreateRandomeResult struct {
	Result pulumi.StringOutput `pulumi:"result"`
}

func (c *Component) CreateRandom(ctx *pulumi.Context, args *ComponentCreateRandomArgs) (*ComponentCreateRandomeResult,
	error) {

	random, err := NewRandom(ctx, "myrandom", &RandomArgs{Length: args.Length}, pulumi.Parent(c))
	if err != nil {
		return nil, err
	}

	return &ComponentCreateRandomeResult{
		Result: random.Result,
	}, nil
}

const providerName = "testcomponent"
const version = "0.0.1"

type module struct {
	version semver.Version
}

func (m *module) Version() semver.Version {
	return m.version
}

func (m *module) Construct(ctx *pulumi.Context, name, typ, urn string) (r pulumi.Resource, err error) {
	switch typ {
	case "testcomponent:index:Component":
		r = &Component{}
	default:
		return nil, fmt.Errorf("unknown resource type: %s", typ)
	}

	err = ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))
	return
}

func main() {
	// Register any resources that can come back as resource references that need to be rehydrated.
	pulumi.RegisterResourceModule("testcomponent", "index", &module{semver.MustParse(version)})

	if err := provider.MainWithOptions(provider.Options{
		Name:    providerName,
		Version: version,
		Construct: func(ctx *pulumi.Context, typ, name string, inputs pulumiprovider.ConstructInputs,
			options pulumi.ResourceOption) (*pulumiprovider.ConstructResult, error) {
			if typ != "testcomponent:index:Component" {
				return nil, fmt.Errorf("unknown resource type %s", typ)
			}
			component, err := NewComponent(ctx, name, options)
			if err != nil {
				return nil, fmt.Errorf("creating component: %w", err)
			}
			return pulumiprovider.NewConstructResult(component)
		},
		Call: func(ctx *pulumi.Context, tok string,
			args pulumiprovider.CallArgs) (*pulumiprovider.CallResult, error) {

			if tok != "testcomponent:index:Component/createRandom" {
				return nil, fmt.Errorf("unknown method %s", tok)
			}

			methodArgs := &ComponentCreateRandomArgs{}
			res, err := args.CopyTo(methodArgs)
			if err != nil {
				return nil, fmt.Errorf("setting args: %w", err)
			}
			component := res.(*Component)

			result, err := component.CreateRandom(ctx, methodArgs)
			if err != nil {
				return nil, fmt.Errorf("calling method: %w", err)
			}

			return pulumiprovider.NewCallResult(result)
		},
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
