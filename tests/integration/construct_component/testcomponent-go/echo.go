// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

// Exposes the Echo resource from the testprovider.
// Requires running `make test_build` and having the built provider on PATH.

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Echo struct {
	pulumi.CustomResourceState
}

func NewEcho(ctx *pulumi.Context, name string, args *EchoArgs, opts ...pulumi.ResourceOption) (*Echo, error) {
	var resource Echo
	if err := ctx.RegisterResource("testprovider:index:Echo", name, args, &resource, opts...); err != nil {
		return nil, err
	}
	return &resource, nil
}

type echoArgs struct {
	Echo interface{} `pulumi:"echo"`
}

type EchoArgs struct {
	Echo pulumi.Input
}

func (EchoArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*echoArgs)(nil)).Elem()
}
