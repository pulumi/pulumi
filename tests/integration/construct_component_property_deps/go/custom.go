// Copyright 2025, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Custom struct {
	pulumi.CustomResourceState

	Value pulumi.StringOutput `pulumi:"value"`
}

func NewCustom(ctx *pulumi.Context,
	name string, args *CustomArgs, opts ...pulumi.ResourceOption,
) (*Custom, error) {
	var resource Custom
	err := ctx.RegisterResource("testcomponent:index:Custom", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type customArgs struct {
	Value string `pulumi:"value"`
}

// The set of arguments for constructing a Custom resource.
type CustomArgs struct {
	Value pulumi.StringInput
}

func (CustomArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*customArgs)(nil)).Elem()
}

type CustomInput interface {
	pulumi.Input

	ToCustomOutput() CustomOutput
	ToCustomOutputWithContext(ctx context.Context) CustomOutput
}

func (*Custom) ElementType() reflect.Type {
	return reflect.TypeOf((**Custom)(nil)).Elem()
}

func (i *Custom) ToCustomOutput() CustomOutput {
	return i.ToCustomOutputWithContext(context.Background())
}

func (i *Custom) ToCustomOutputWithContext(ctx context.Context) CustomOutput {
	return pulumi.ToOutputWithContext(ctx, i).(CustomOutput)
}

type CustomOutput struct{ *pulumi.OutputState }

func (CustomOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Custom)(nil)).Elem()
}

func (o CustomOutput) ToCustomOutput() CustomOutput {
	return o
}

func (o CustomOutput) ToCustomOutputWithContext(ctx context.Context) CustomOutput {
	return o
}

func (o CustomOutput) Value() pulumi.StringOutput {
	return o.ApplyT(func(v *Custom) pulumi.StringOutput { return v.Value }).(pulumi.StringOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*CustomInput)(nil)).Elem(), &Custom{})
	pulumi.RegisterOutputType(CustomOutput{})
}
