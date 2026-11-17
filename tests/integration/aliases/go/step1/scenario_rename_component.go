// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type renameCompFooResource struct {
	pulumi.ResourceState
}

type renameCompFooComponent struct {
	pulumi.ResourceState
}

func newRenameCompFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*renameCompFooResource, error) {
	fooRes := &renameCompFooResource{}
	err := ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

// Scenario #3 - rename a component (and all it's children)
func newRenameCompFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*renameCompFooComponent, error) {
	fooComp := &renameCompFooComponent{}
	err := ctx.RegisterComponentResource("my:module:FooComponent42", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	// Note that both un-prefixed and parent-name-prefixed child names are supported. For the later, the implicit
	// alias inherited from the parent alias will include replacing the name prefix to match the parent alias name.
	parentOpt := pulumi.Parent(fooComp)
	_, err = newRenameCompFooResource(ctx, name+"-child", parentOpt)
	if err != nil {
		return nil, err
	}
	_, err = newRenameCompFooResource(ctx, "otherchild", parentOpt)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func scenarioRenameComponent(ctx *pulumi.Context) error {
	_, err := newRenameCompFooComponent(ctx, "comp3")
	return err
}
