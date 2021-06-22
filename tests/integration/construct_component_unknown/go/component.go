// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type componentArgs struct {
	Message string              `pulumi:"message"`
	Nested  componentNestedArgs `pulumi:"nested"`
}

type ComponentArgs struct {
	Message pulumi.StringInput
	Nested  ComponentNestedInput
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

type componentNestedArgs struct {
	Value string `pulumi:"Value"`
}

type ComponentNestedArgs struct {
	Value pulumi.StringInput
}

type ComponentNestedInput interface {
	pulumi.Input
}

func (ComponentNestedArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentNestedArgs)(nil)).Elem()
}

type Component struct {
	pulumi.ResourceState
}

func NewComponent(
	ctx *pulumi.Context, name string, args *ComponentArgs, opts ...pulumi.ResourceOption) (*Component, error) {

	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}

	return &resource, nil
}
