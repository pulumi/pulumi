// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Component struct {
	pulumi.ResourceState
}

func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, nil, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func (c *Component) CreateRandom(ctx *pulumi.Context, args *ComponentCreateRandomArgs) (ComponentCreateRandomResultOutput, error) {
	out, err := ctx.Call("testcomponent:index:Component/createRandom", args, ComponentCreateRandomResultOutput{}, c)
	if err != nil {
		return ComponentCreateRandomResultOutput{}, err
	}
	return out.(ComponentCreateRandomResultOutput), nil
}

type componentCreateRandomArgs struct {
	Length int `pulumi:"length"`
}

type ComponentCreateRandomArgs struct {
	Length pulumi.IntInput
}

func (ComponentCreateRandomArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentCreateRandomArgs)(nil)).Elem()
}

type ComponentCreateRandomResult struct {
	Result string `pulumi:"result"`
}

type ComponentCreateRandomResultOutput struct{ *pulumi.OutputState }

func (ComponentCreateRandomResultOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ComponentCreateRandomResult)(nil)).Elem()
}

func (o ComponentCreateRandomResultOutput) Result() pulumi.StringOutput {
	return o.ApplyT(func(v ComponentCreateRandomResult) string { return v.Result }).(pulumi.StringOutput)
}

func init() {
	pulumi.RegisterOutputType(ComponentCreateRandomResultOutput{})
}
