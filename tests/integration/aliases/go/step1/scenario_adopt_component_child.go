// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type adoptChildFooResource struct {
	pulumi.ResourceState
}

type adoptChildFooComponent struct {
	pulumi.ResourceState
}

func newAdoptChildFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*adoptChildFooResource, error) {
	fooRes := &adoptChildFooResource{}
	err := ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

func newAdoptChildFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*adoptChildFooComponent, error) {
	fooComp := &adoptChildFooComponent{}
	err := ctx.RegisterComponentResource("my:module:FooComponent", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func scenarioAdoptComponentChild(ctx *pulumi.Context) error {
	_, err := newAdoptChildFooComponent(ctx, "compAdoptChild")
	if err != nil {
		return err
	}

	_, err = newAdoptChildFooResource(ctx, "childAdoptChild")
	return err
}
