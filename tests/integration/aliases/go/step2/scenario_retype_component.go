// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type retypeFooResource struct {
	pulumi.ResourceState
}

type retypeFooComponent struct {
	pulumi.ResourceState
}

func newRetypeFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*retypeFooResource, error) {
	fooRes := &retypeFooResource{}
	err := ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

// Scenario #4 - change the type of a component
func newRetypeFooComponent(ctx *pulumi.Context, name string) (*retypeFooComponent, error) {
	fooComp := &retypeFooComponent{}
	alias := &pulumi.Alias{
		Type: pulumi.StringInput(pulumi.String("my:module:FooComponent44")),
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	err := ctx.RegisterComponentResource("my:diffmodule:FooComponent55DiffType", name, fooComp, aliasOpt)
	if err != nil {
		return nil, err
	}
	parentOpt := pulumi.Parent(fooComp)
	_, err = newRetypeFooResource(ctx, "otherchild", parentOpt)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func scenarioRetypeComponent(ctx *pulumi.Context) error {
	_, err := newRetypeFooComponent(ctx, "comp4")
	return err
}
