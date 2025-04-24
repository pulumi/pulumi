// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type FailsOnCreate struct {
	pulumi.CustomResourceState

	Value pulumi.Float64Output `pulumi:"value"`
}

func NewFailsOnCreate(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*FailsOnCreate, error) {
	var resource FailsOnCreate
	err := ctx.RegisterResource("testprovider:index:FailsOnCreate", name, nil, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("xyz", pulumi.String("DEF"))
		res, _ := NewFailsOnCreate(ctx, "test")
		ctx.Export("foo", res.Value)
		return nil
	})
}
