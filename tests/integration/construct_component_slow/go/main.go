// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Component struct {
	pulumi.ResourceState
}

func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, nil, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		if _, err := NewComponent(ctx, "a"); err != nil {
			return err
		}
		return nil
	})
}
