package main

import (
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type SimpleComponentArgs struct {
}

type SimpleComponent struct {
	pulumi.ResourceState
}

func NewSimpleComponent(
	ctx *pulumi.Context,
	name string,
	args *SimpleComponentArgs,
	opts ...pulumi.ResourceOption,
) (*SimpleComponent, error) {
	var componentResource SimpleComponent
	err := ctx.RegisterComponentResource("components:index:SimpleComponent", name, &componentResource, opts...)
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
	_, err = random.NewRandomPassword(ctx, fmt.Sprintf("%s-secondPassword", name), &random.RandomPasswordArgs{
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
