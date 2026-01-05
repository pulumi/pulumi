// Copyright 2025, Pulumi Corporation.  All rights reserved.

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Echo struct {
	pulumi.CustomResourceState

	Echo pulumi.StringOutput `pulumi:"echo"`
}

type echoArgs struct {
	Echo string `pulumi:"echo"`
}

type EchoArgs struct {
	Echo pulumi.StringInput
}

func (EchoArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*echoArgs)(nil)).Elem()
}

func NewEcho(ctx *pulumi.Context, name string, args *EchoArgs, opts ...pulumi.ResourceOption) (*Echo, error) {
	var resource Echo
	err := ctx.RegisterResource("testprovider:index:Echo", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}
