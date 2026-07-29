// Copyright 2026, Pulumi Corporation.  All rights reserved.
//go:build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type adoptFooResource struct {
	pulumi.ResourceState
}

type adoptFooComponent struct {
	pulumi.ResourceState
}

type adoptFooComponent2 struct {
	pulumi.ResourceState
}

type adoptFooComponent3 struct {
	pulumi.ResourceState
}

type adoptFooComponent4 struct {
	pulumi.ResourceState
}

func newAdoptFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*adoptFooResource, error) {
	fooRes := &adoptFooResource{}
	err := ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	if err != nil {
		return nil, err
	}
	return fooRes, nil
}

func newAdoptFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*adoptFooComponent, error) {
	fooComp := &adoptFooComponent{}
	err := ctx.RegisterComponentResource("my:module:FooComponent", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	var nilInput pulumi.StringInput
	aliasURN := pulumi.CreateURN(
		pulumi.StringInput(pulumi.String("res2")),
		pulumi.StringInput(pulumi.String("my:module:FooResource")),
		nilInput,
		pulumi.StringInput(pulumi.String(ctx.Project())),
		pulumi.StringInput(pulumi.String(ctx.Stack())))
	alias := &pulumi.Alias{
		URN: aliasURN,
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	parentOpt := pulumi.Parent(fooComp)
	_, err = newAdoptFooResource(ctx, name+"-child", aliasOpt, parentOpt)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func newAdoptFooComponent2(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*adoptFooComponent2, error) {
	fooComp := &adoptFooComponent2{}
	err := ctx.RegisterComponentResource("my:module:FooComponent2", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func newAdoptFooComponent3(ctx *pulumi.Context,
	name string,
	childAliasParent pulumi.Resource,
	opts ...pulumi.ResourceOption,
) (*adoptFooComponent3, error) {
	fooComp := &adoptFooComponent3{}
	err := ctx.RegisterComponentResource("my:module:FooComponent3", name, fooComp, opts...)
	if err != nil {
		return nil, err
	}

	alias := &pulumi.Alias{}
	if childAliasParent != nil {
		alias.Parent = childAliasParent
	} else {
		alias.NoParent = pulumi.Bool(true)
	}

	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	parentOpt := pulumi.Parent(fooComp)
	_, err = newAdoptFooComponent2(ctx, name+"-child", aliasOpt, parentOpt)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

func newAdoptFooComponent4(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*adoptFooComponent4, error) {
	fooComp := &adoptFooComponent4{}
	alias := &pulumi.Alias{
		Parent: nil,
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias, *alias})
	o := []pulumi.ResourceOption{aliasOpt}
	o = append(o, opts...)
	err := ctx.RegisterComponentResource("my:module:FooComponent4", name, fooComp, o...)
	if err != nil {
		return nil, err
	}
	return fooComp, nil
}

// Scenario #2 - adopt a resource into a component (and inner scenarios 3-5)
func scenarioAdoptIntoComponent(ctx *pulumi.Context) error {
	comp2, err := newAdoptFooComponent(ctx, "comp2")
	if err != nil {
		return err
	}
	alias := &pulumi.Alias{
		NoParent: pulumi.Bool(true),
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	parentOpt := pulumi.Parent(comp2)
	_, err = newAdoptFooComponent2(ctx, "unparented", aliasOpt, parentOpt)
	if err != nil {
		return err
	}
	_, err = newAdoptFooComponent3(ctx, "parentedbystack", nil)
	if err != nil {
		return err
	}
	pbcOpt := pulumi.Parent(comp2)
	_, err = newAdoptFooComponent3(ctx, "parentedbycomponent", comp2, pbcOpt)
	if err != nil {
		return err
	}
	dupeOpt := pulumi.Parent(comp2)
	_, err = newAdoptFooComponent4(ctx, "duplicateAliases", dupeOpt)
	if err != nil {
		return err
	}
	return nil
}
