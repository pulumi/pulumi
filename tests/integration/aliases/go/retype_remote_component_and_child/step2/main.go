// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type BucketComponentV2 struct {
	pulumi.ResourceState
}

func NewBucketComponentV2(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*BucketComponentV2, error) {
	component := &BucketComponentV2{}
	err := ctx.RegisterRemoteComponentResource("wibble:index:BucketComponentV2", name, nil, component, opts...)
	if err != nil {
		return nil, err
	}
	return component, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewBucketComponentV2(ctx, "main-bucket")
		return err
	})
}
