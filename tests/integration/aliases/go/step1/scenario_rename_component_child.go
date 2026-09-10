// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type renameChildFooResource struct {
	pulumi.ResourceState
}

type renameChildFooComponent struct {
	pulumi.ResourceState
}

func newRenameChildFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*renameChildFooResource, error) {
	fooRes := &renameChildFooResource{}
	err := ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

func newRenameChildFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*renameChildFooComponent, error) {
	fooComp := &renameChildFooComponent{}
	err := ctx.RegisterComponentResource("my:module:FooComponent", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	parentOpt := pulumi.Parent(fooComp)
	_, err = newRenameChildFooResource(ctx, "childRenameChild", parentOpt)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func scenarioRenameComponentChild(ctx *pulumi.Context) error {
	_, err := newRenameChildFooComponent(ctx, "compRenameChild")
	return err
}
