package main

import (
	"fmt"

	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type PrimitiveComponentArgs struct {
	NumberArray pulumi.Float64ArrayInput
	BooleanMap  pulumi.BoolMapInput
}

type PrimitiveComponent struct {
	pulumi.ResourceState
}

func NewPrimitiveComponent(
	ctx *pulumi.Context,
	name string,
	args *PrimitiveComponentArgs,
	opts ...pulumi.ResourceOption,
) (*PrimitiveComponent, error) {
	var componentResource PrimitiveComponent
	err := ctx.RegisterComponentResource("components:index:PrimitiveComponent", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	_, err = primitive.NewResource(ctx, fmt.Sprintf("%s-res", name), &primitive.ResourceArgs{
		Boolean:     pulumi.Bool(true),
		Float:       pulumi.Float64(3.5),
		Integer:     pulumi.Int(3),
		String:      pulumi.String("plain"),
		NumberArray: args.NumberArray,
		BooleanMap:  args.BooleanMap,
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{})
	if err != nil {
		return nil, err
	}
	return &componentResource, nil
}
