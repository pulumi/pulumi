// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Resource struct {
	pulumi.ResourceState
}

type ComponentSix struct {
	pulumi.ResourceState
}

type ComponentSixParent struct {
	pulumi.ResourceState
}

func NewResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Resource, error) {
	comp := &Resource{}
	err := ctx.RegisterComponentResource("my:module:Resource", name, comp, opts...)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

// Scenario #6 - Nested parents changing types
func NewComponentSix(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*ComponentSix, error) {
	comp := &ComponentSix{}
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
	_, err = NewResource(ctx, "otherchild", parentOpt)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

func NewComponentSixParent(ctx *pulumi.Context, name string,
	opts ...pulumi.ResourceOption,
) (*ComponentSixParent, error) {
	comp := &ComponentSixParent{}
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
	_, err = NewComponentSix(ctx, "child", parentOpt)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewComponentSixParent(ctx, "comp6")
		if err != nil {
			return err
		}

		return nil
	})
}
