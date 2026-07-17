// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type retypeParentsResource struct {
	pulumi.ResourceState
}

type retypeParentsComponentSix struct {
	pulumi.ResourceState
}

type retypeParentsComponentSixParent struct {
	pulumi.ResourceState
}

func newRetypeParentsResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*retypeParentsResource, error) {
	comp := &retypeParentsResource{}
	err := ctx.RegisterComponentResource("my:module:Resource", name, comp, opts...)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

// Scenario #6 - Nested parents changing types
func newRetypeParentsComponentSix(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*retypeParentsComponentSix, error) {
	comp := &retypeParentsComponentSix{}
	aliases := make([]pulumi.Alias, 0)
	for i := 0; i < 100; i++ {
		aliases = append(aliases, pulumi.Alias{
			Type: pulumi.Sprintf("my:module:ComponentSix-v%d", i),
		})
	}
	err := ctx.RegisterComponentResource(
		"my:module:ComponentSix-v0", name, comp,
		pulumi.Aliases(aliases), pulumi.Composite(opts...))
	if err != nil {
		return nil, err
	}
	parentOpt := pulumi.Parent(comp)
	_, err = newRetypeParentsResource(ctx, "otherchild", parentOpt)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

func newRetypeParentsComponentSixParent(ctx *pulumi.Context, name string,
	opts ...pulumi.ResourceOption,
) (*retypeParentsComponentSixParent, error) {
	comp := &retypeParentsComponentSixParent{}
	aliases := make([]pulumi.Alias, 0)
	for i := 0; i < 10; i++ {
		aliases = append(aliases, pulumi.Alias{
			Type: pulumi.Sprintf("my:module:ComponentSixParent-v%d", i),
		})
	}
	err := ctx.RegisterComponentResource(
		"my:module:ComponentSixParent-v0", name, comp,
		pulumi.Aliases(aliases), pulumi.Composite(opts...))
	if err != nil {
		return nil, err
	}
	parentOpt := pulumi.Parent(comp)
	_, err = newRetypeParentsComponentSix(ctx, "child", parentOpt)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

func scenarioRetypeParents(ctx *pulumi.Context) error {
	_, err := newRetypeParentsComponentSixParent(ctx, "comp6")
	return err
}
