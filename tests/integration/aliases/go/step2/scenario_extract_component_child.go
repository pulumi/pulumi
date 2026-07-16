// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type extractChildFooResource struct {
	pulumi.ResourceState
}

type extractChildFooComponent struct {
	pulumi.ResourceState
}

func newExtractChildFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*extractChildFooResource, error) {
	fooRes := &extractChildFooResource{}
	err := ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

func newExtractChildFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*extractChildFooComponent, error) {
	fooComp := &extractChildFooComponent{}
	err := ctx.RegisterComponentResource("my:module:FooComponent", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func scenarioExtractComponentChild(ctx *pulumi.Context) error {
	foo, err := newExtractChildFooComponent(ctx, "compExtractChild")
	if err != nil {
		return err
	}

	aliasOpt := pulumi.Aliases([]pulumi.Alias{{
		Parent: foo,
	}})
	_, err = newExtractChildFooResource(ctx, "childExtractChild", aliasOpt)
	return err
}
