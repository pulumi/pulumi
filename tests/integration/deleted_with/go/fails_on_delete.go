// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

// Exposes the FailsOnDelete resource from the testprovider.

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type FailsOnDelete struct {
	pulumi.CustomResourceState
}

func NewFailsOnDelete(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*FailsOnDelete, error) {
	var resource FailsOnDelete
	err := ctx.RegisterResource("testprovider:index:FailsOnDelete", name, nil, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}
