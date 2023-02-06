// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type BucketComponent struct {
	pulumi.ResourceState
}

func NewBucketComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*BucketComponent, error) {
	component := &BucketComponent{}
	err := ctx.RegisterRemoteComponentResource("wibble:index:BucketComponent", name, nil, component, opts...)
	if err != nil {
		return nil, err
	}
	return component, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewBucketComponent(ctx, "main-bucket")
		return err
	})
}
