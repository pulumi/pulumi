package main

import (
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type AnotherComponentArgs struct {
}

type AnotherComponent struct {
	pulumi.ResourceState
}

func NewAnotherComponent(
	ctx *pulumi.Context,
	name string,
	args *AnotherComponentArgs,
	opts ...pulumi.ResourceOption,
) (*AnotherComponent, error) {
	var componentResource AnotherComponent
	err := ctx.RegisterComponentResource("components:index:AnotherComponent", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	_, err = random.NewRandomPassword(ctx, fmt.Sprintf("%s-firstPassword", name), &random.RandomPasswordArgs{
		Length:  pulumi.Int(16),
		Special: true,
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
