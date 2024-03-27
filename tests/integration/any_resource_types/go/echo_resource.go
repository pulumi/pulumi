// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

// Exposes the FailsOnDelete resource from the testprovider.

package main

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Echo struct {
	pulumi.CustomResourceState

	Echo pulumi.ResourceOutput `pulumi:"echo"`
}

func NewEcho(ctx *pulumi.Context, name string, args *EchoArgs, opts ...pulumi.ResourceOption) (*Echo, error) {
	var resource Echo
	err := ctx.RegisterResource("testprovider:index:Echo", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

// Input properties used for looking up and filtering Echo resources.
type echoState struct {
}

type EchoState struct {
}

func (EchoState) ElementType() reflect.Type {
	return reflect.TypeOf((*echoState)(nil)).Elem()
}

type echoArgs struct {
	Echo pulumi.Resource `pulumi:"echo"`
}

type EchoArgs struct {
	Echo pulumi.ResourceInput
}

func (EchoArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*echoArgs)(nil)).Elem()
}

type EchoInput interface {
	pulumi.Input

	ToEchoOutput() EchoOutput
	ToEchoOutputWithContext(ctx context.Context) EchoOutput
}

func (*Echo) ElementType() reflect.Type {
	return reflect.TypeOf((**Echo)(nil)).Elem()
}

func (i *Echo) ToEchoOutput() EchoOutput {
	return i.ToEchoOutputWithContext(context.Background())
}

func (i *Echo) ToEchoOutputWithContext(ctx context.Context) EchoOutput {
	return pulumi.ToOutputWithContext(ctx, i).(EchoOutput)
}

type EchoOutput struct{ *pulumi.OutputState }

func (EchoOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Echo)(nil)).Elem()
}

func (o EchoOutput) ToEchoOutput() EchoOutput {
	return o
}

func (o EchoOutput) ToEchoOutputWithContext(ctx context.Context) EchoOutput {
	return o
}

func (o EchoOutput) Echo() pulumi.ResourceOutput {
	return o.ApplyT(func(v *Echo) pulumi.ResourceOutput { return v.Echo }).(pulumi.ResourceOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*EchoInput)(nil)).Elem(), &Echo{})
	pulumi.RegisterOutputType(EchoOutput{})
}
