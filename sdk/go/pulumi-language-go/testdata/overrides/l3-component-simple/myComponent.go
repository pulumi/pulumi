package main

import (
	"fmt"
	"reflect"

	"example.com/pulumi-simple/sdk/go/v2/simple"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type myComponentArgs struct {
	Input bool `pulumi:"input"`
}

type MyComponentArgs struct {
	Input pulumi.BoolInput
}

func (MyComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*myComponentArgs)(nil)).Elem()
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
	err := ctx.RegisterComponentResourceV2("components:index:MyComponent", name, args, &componentResource, opts...)
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
