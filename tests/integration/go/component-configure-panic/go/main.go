//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewComponent(ctx, "foo")
		return err
	})
}

type Component struct {
	pulumi.ResourceState
}

func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	var component Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, nil, &component, opts...)
	return &component, err
}
