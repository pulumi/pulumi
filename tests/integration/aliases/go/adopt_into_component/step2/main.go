// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

// FooComponent is a component resource
type FooResource struct {
	pulumi.ResourceState
}

type FooComponent struct {
	pulumi.ResourceState
}

type FooComponent2 struct {
	pulumi.ResourceState
}

type FooComponent3 struct {
	pulumi.ResourceState
}

type FooComponent4 struct {
	pulumi.ResourceState
}

func NewFooResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) *FooResource {
	fooRes := &FooResource{}
	ctx.RegisterComponentResource("my:module:FooResource", name, fooRes, opts...)
	return fooRes
}

func NewFooComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) *FooComponent {
	fooComp := &FooComponent{}
	ctx.RegisterComponentResource("my:module:FooComponent", name, fooComp, opts...)
	var nilInput pulumi.StringInput
	aliasURN := pulumi.CreateURN(pulumi.StringInput(pulumi.String("res2")), pulumi.StringInput(pulumi.String("my:module:FooResource")), nilInput, nilInput, nilInput)
	alias := &pulumi.Alias{
		URN: aliasURN,
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	parentOpt := pulumi.Parent(fooComp)
	NewFooResource(ctx, name+"-child", aliasOpt, parentOpt)
	return fooComp
}

func NewFooComponent2(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) *FooComponent2 {
	fooComp := &FooComponent2{}
	ctx.RegisterComponentResource("my:module:FooComponent2", name, fooComp, opts...)
	return fooComp
}

func NewFooComponent3(ctx *pulumi.Context, name string, childAliasParent pulumi.Resource, opts ...pulumi.ResourceOption) *FooComponent3 {
	fooComp := &FooComponent3{}
	ctx.RegisterComponentResource("my:module:FooComponent3", name, fooComp, opts...)

	alias := &pulumi.Alias{
		Parent: childAliasParent,
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
	parentOpt := pulumi.Parent(fooComp)
	NewFooComponent2(ctx, name+"-child", aliasOpt, parentOpt)
	return fooComp
}

func NewFooComponent4(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) *FooComponent4 {
	fooComp := &FooComponent4{}
	alias := &pulumi.Alias{
		Parent: nil,
	}
	aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias, *alias})
	o := []pulumi.ResourceOption{aliasOpt}
	o = append(o, opts...)
	ctx.RegisterComponentResource("my:module:FooComponent4", name, fooComp, o...)
	return fooComp
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		comp2 := NewFooComponent(ctx, "comp2", nil)
		alias := &pulumi.Alias{
			Parent: nil,
		}
		aliasOpt := pulumi.Aliases([]pulumi.Alias{*alias})
		parentOpt := pulumi.Parent(comp2)
		_ = NewFooComponent2(ctx, "unparented", aliasOpt, parentOpt)
		_ = NewFooComponent3(ctx, "parentedbystack", nil)
		pbcOpt := pulumi.Parent(comp2)
		_ = NewFooComponent3(ctx, "parentedbycomponent", comp2, pbcOpt)
		dupeOpt := pulumi.Parent(comp2)
		_ = NewFooComponent4(ctx, "duplicateAliases", dupeOpt)

		return nil
	})
}
