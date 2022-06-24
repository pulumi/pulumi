// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"errors"
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

type ComponentArgs struct {
	Bar BarPtrInput `pulumi:"bar"`
	Foo *FooArgs    `pulumi:"foo"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs,
	opts ...pulumi.ResourceOption) (*Component, error) {
	if args == nil {
		return nil, errors.New("args is required")
	}

	if args.Foo == nil {
		return nil, errors.New(`expected args.Foo to be non-nil`)
	}
	if args.Foo.Something == nil {
		return nil, errors.New(`expected args.Foo.Something to be non-nil`)
	}
	something, somethingIsString := args.Foo.Something.(pulumi.String)
	if !somethingIsString {
		return nil, errors.New(`expected args.Foo.Something to be pulumi.String`)
	}
	if something != "hello" {
		return nil, fmt.Errorf(`expected args.Foo.Something to equal "hello" but got %q`, something)
	}

	barArgs, isBarArgs := args.Bar.(BarArgs)
	if !isBarArgs {
		return nil, errors.New("expected args.Bar to be BarArgs")
	}
	tags, isStringMap := barArgs.Tags.(pulumi.StringMap)
	if !isStringMap {
		return nil, errors.New("expected args.Bar.Tags to be pulumi.StringMap")
	}

	a, aIsString := tags["a"].(pulumi.String)
	if !aIsString {
		return nil, errors.New(`expected args.Bar.Tags["a"] to be pulumi.String`)
	}
	if a != "world" {
		return nil, fmt.Errorf(`expected args.Bar.Tags["a"] to equal "world" but got %q`, a)
	}

	b, bIsStringOutput := tags["b"].(pulumi.StringOutput)
	if !bIsStringOutput {
		return nil, errors.New(`expected args.Bar.Tags["b"] to be pulumi.StringOutput`)
	}
	b.ApplyT(func(v string) (string, error) {
		if v != "shh" {
			return v, fmt.Errorf(`expected args.Bar.Tags["b"] to equal "shh" but got %q`, v)
		}
		return v, nil
	})

	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{}); err != nil {
		return nil, err
	}

	return component, nil
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

			args := &ComponentArgs{}
			if err := inputs.CopyTo(args); err != nil {
				return nil, fmt.Errorf("setting args: %w", err)
			}

			component, err := NewComponent(ctx, name, args, options)
			if err != nil {
				return nil, fmt.Errorf("creating component: %w", err)
			}

			return pulumiprovider.NewConstructResult(component)
		},
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
