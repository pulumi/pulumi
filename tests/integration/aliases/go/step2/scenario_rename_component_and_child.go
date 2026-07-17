// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type renameBothFooResource struct {
	pulumi.ResourceState
}

type renameBothFooComponent struct {
	pulumi.ResourceState
}

func newRenameBothFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*renameBothFooResource, error) {
	fooRes := &renameBothFooResource{}
	err := ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

// Scenario #5 - composing #1 and #3 and making both changes at the same time
func newRenameBothFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*renameBothFooComponent, error) {
	fooComp := &renameBothFooComponent{}
	err := ctx.RegisterComponentResource("my:module:FooComponent43", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	parentOpt := pulumi.Parent(fooComp)
	alias := &pulumi.Alias{
		Name:   pulumi.StringInput(pulumi.String("otherchild")),
		Parent: fooComp,
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	_, err = newRenameBothFooResource(ctx, "otherchildrenamed", parentOpt, aliasOpt)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func scenarioRenameComponentAndChild(ctx *pulumi.Context) error {
	alias := &pulumi.Alias{Name: pulumi.StringInput(pulumi.String("comp5"))}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	_, err := newRenameBothFooComponent(ctx, "newcomp5", aliasOpt)
	return err
}
