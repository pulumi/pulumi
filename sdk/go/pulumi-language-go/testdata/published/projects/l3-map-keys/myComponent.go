package main

import (
	"fmt"

	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type MyComponentArgs struct {
	BooleanMap map[string]pulumi.BoolInput
}

type MyComponent struct {
	pulumi.ResourceState
	BooleanMap pulumi.AnyOutput
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
	res, err := primitive.NewResource(ctx, fmt.Sprintf("%s-res", name), &primitive.ResourceArgs{
		Boolean: pulumi.Bool(false),
		Float:   pulumi.Float64(2.17),
		Integer: pulumi.Int(-12),
		String:  pulumi.String("adversarial"),
		NumberArray: pulumi.Float64Array{
			pulumi.Float64(0),
			pulumi.Float64(1),
		},
		BooleanMap: args.BooleanMap,
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{
		"booleanMap": res.BooleanMap,
	})
	if err != nil {
		return nil, err
	}
	componentResource.BooleanMap = pulumi.Any(res.BooleanMap)
	return &componentResource, nil
}
