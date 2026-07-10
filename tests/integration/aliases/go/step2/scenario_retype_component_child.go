// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type retypeChildFooResource struct {
	pulumi.ResourceState
}

type retypeChildFooComponent struct {
	pulumi.ResourceState
}

func newRetypeChildFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*retypeChildFooResource, error) {
	fooRes := &retypeChildFooResource{}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{{
		Type: pulumi.String("my:module:FooResource"),
	}})
	opts = append(opts, aliasOpt)
	err := ctx.RegisterComponentResource("my:module:FooResourceNew", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

func newRetypeChildFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*retypeChildFooComponent, error) {
	fooComp := &retypeChildFooComponent{}
	err := ctx.RegisterComponentResource("my:module:FooComponent", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	parentOpt := pulumi.Parent(fooComp)
	_, err = newRetypeChildFooResource(ctx, "childRetypeChild", parentOpt)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func scenarioRetypeComponentChild(ctx *pulumi.Context) error {
	_, err := newRetypeChildFooComponent(ctx, "compRetypeChild")
	return err
}
