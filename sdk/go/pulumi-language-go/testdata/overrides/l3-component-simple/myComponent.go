package main

import (
	"fmt"

	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type MyComponentArgs struct {
	Input pulumi.BoolInput
}

type MyComponent struct {
	pulumi.ResourceState
	Output pulumi.AnyOutput
}

func NewMyComponent(
	ctx *pulumi.Context,
	name string,
	args *MyComponentArgs,
	opts ...pulumi.ResourceOption,
) (*MyComponent, error) {
	var componentResource MyComponent
	err := ctx.RegisterComponentResource("components:index:MyComponent", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	res, err := simple.NewResource(ctx, fmt.Sprintf("%s-res", name), &simple.ResourceArgs{
		Value: args.Input,
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{
		"output": res.Value,
	})
	if err != nil {
		return nil, err
	}
	componentResource.Output = res.Value.ApplyT(func(v interface{}) interface{} {
		return v
	}).(pulumi.AnyOutput)
	return &componentResource, nil
}
