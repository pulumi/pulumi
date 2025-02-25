// Copyright 2025, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Component struct {
	pulumi.ResourceState

	PropertyDeps pulumi.StringArrayMapOutput `pulumi:"propertyDeps"`
}

func NewComponent(ctx *pulumi.Context,
	name string, args *ComponentArgs, opts ...pulumi.ResourceOption,
) (*Component, error) {
	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type componentArgs struct {
	Resource     *Custom            `pulumi:"resource"`
	ResourceList []*Custom          `pulumi:"resourceList"`
	ResourceMap  map[string]*Custom `pulumi:"resourceMap"`
}

type ComponentArgs struct {
	Resource     *Custom
	ResourceList []*Custom
	ResourceMap  map[string]*Custom
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

func (r *Component) Refs(ctx *pulumi.Context, args *ComponentRefsArgs) (ComponentRefsResultOutput, error) {
	out, err := ctx.Call("testcomponent:index:Component/refs", args, ComponentRefsResultOutput{}, r)
	if err != nil {
		return ComponentRefsResultOutput{}, err
	}
	return out.(ComponentRefsResultOutput), nil
}

type componentRefsArgs struct {
	Resource     *Custom            `pulumi:"resource"`
	ResourceList []*Custom          `pulumi:"resourceList"`
	ResourceMap  map[string]*Custom `pulumi:"resourceMap"`
}

// The set of arguments for the Refs method of the Component resource.
type ComponentRefsArgs struct {
	Resource     CustomInput
	ResourceList []CustomInput
	ResourceMap  map[string]CustomInput
}

func (ComponentRefsArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentRefsArgs)(nil)).Elem()
}

type ComponentRefsResult struct {
	Result map[string][]string `pulumi:"result"`
}

type ComponentRefsResultOutput struct{ *pulumi.OutputState }

func (ComponentRefsResultOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ComponentRefsResult)(nil)).Elem()
}

func (o ComponentRefsResultOutput) Result() pulumi.StringArrayMapOutput {
	return o.ApplyT(func(v ComponentRefsResult) map[string][]string { return v.Result }).(pulumi.StringArrayMapOutput)
}

type ComponentInput interface {
	pulumi.Input

	ToComponentOutput() ComponentOutput
	ToComponentOutputWithContext(ctx context.Context) ComponentOutput
}

func (*Component) ElementType() reflect.Type {
	return reflect.TypeOf((**Component)(nil)).Elem()
}

func (i *Component) ToComponentOutput() ComponentOutput {
	return i.ToComponentOutputWithContext(context.Background())
}

func (i *Component) ToComponentOutputWithContext(ctx context.Context) ComponentOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentOutput)
}

type ComponentOutput struct{ *pulumi.OutputState }

func (ComponentOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Component)(nil)).Elem()
}

func (o ComponentOutput) ToComponentOutput() ComponentOutput {
	return o
}

func (o ComponentOutput) ToComponentOutputWithContext(ctx context.Context) ComponentOutput {
	return o
}

func (o ComponentOutput) PropertyDeps() pulumi.StringArrayMapOutput {
	return o.ApplyT(func(v *Component) pulumi.StringArrayMapOutput { return v.PropertyDeps }).(pulumi.StringArrayMapOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*ComponentInput)(nil)).Elem(), &Component{})
	pulumi.RegisterOutputType(ComponentOutput{})
	pulumi.RegisterOutputType(ComponentRefsResultOutput{})
}
