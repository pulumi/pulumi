// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

func main() {
	if err := provider.ComponentMain("testcomponent", "0.0.1", nil /* schema */, construct); err != nil {
		cmdutil.Exit(err)
	}
}

func construct(
	ctx *pulumi.Context,
	typ, name string,
	inputs pulumiprovider.ConstructInputs,
	options pulumi.ResourceOption,
) (*pulumiprovider.ConstructResult, error) {
	if typ != "testcomponent:index:Component" {
		return nil, fmt.Errorf("unknown resource type %q", typ)
	}

	comp, err := NewComponent(ctx, name, options)
	if err != nil {
		return nil, err
	}

	return pulumiprovider.NewConstructResult(comp)
}

// Component is a component resource.
//
// It's exposed to other SDKs from 'construct' above.
type Component struct {
	pulumi.ResourceState

	Result pulumi.StringOutput `pulumi:"result"`
}

// NewComponent builds a new component resource with the given name.
//
// It will instantiate a random resource as a child of the component
// with the same name.
func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	var comp Component
	if err := ctx.RegisterComponentResource("testcomponent:index:Component", name, &comp, opts...); err != nil {
		return nil, err
	}

	r, err := NewRandom(ctx, name, &RandomArgs{Length: pulumi.Int(10)}, pulumi.Parent(&comp))
	if err != nil {
		return nil, err
	}

	comp.Result = r.Result
	return &comp, ctx.RegisterResourceOutputs(&comp, pulumi.Map{
		"result": comp.Result,
	})
}

// Random is a custom resource that generates a random string.
//
// It's implemented in the tests/testprovider directory.
// This is a Go-level reference to that resource.
type Random struct {
	pulumi.CustomResourceState

	Length pulumi.IntOutput    `pulumi:"length"`
	Result pulumi.StringOutput `pulumi:"result"`
}

// NewRandom builds a new random resource with the given name.
func NewRandom(ctx *pulumi.Context,
	name string, args *RandomArgs, opts ...pulumi.ResourceOption,
) (*Random, error) {
	if args == nil || args.Length == nil {
		return nil, errors.New("missing required argument 'Length'")
	}

	var resource Random
	err := ctx.RegisterResource("testprovider:index:Random", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}

	return &resource, nil
}

// RandomArgs specifies the parameters for a Random resource.
type RandomArgs struct {
	// Length of the random string to generate.
	Length pulumi.IntInput
}

// ElementType implements the pulumi.Input interface.
func (RandomArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*randomArgs)(nil)).Elem()
}

type randomArgs struct {
	Length int `pulumi:"length"`
}
