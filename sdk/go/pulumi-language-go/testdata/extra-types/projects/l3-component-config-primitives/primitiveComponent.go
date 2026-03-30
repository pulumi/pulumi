package main

import (
	"fmt"

	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type PrimitiveComponentArgs struct {
	Boolean pulumi.BoolInput
	Float   pulumi.Float64Input
	Integer pulumi.IntInput
	String  pulumi.StringInput
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
		Boolean: args.Boolean,
		Float:   args.Float,
		Integer: args.Integer,
		String:  args.String,
		NumberArray: pulumi.Float64Array{
			pulumi.Float64(-1),
			pulumi.Float64(0),
			pulumi.Float64(1),
		},
		BooleanMap: pulumi.BoolMap{
			"t": pulumi.Bool(true),
			"f": pulumi.Bool(false),
		},
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
