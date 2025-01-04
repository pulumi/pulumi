package main

import (
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type FirstArgs struct {
	PasswordLength pulumi.Float64Input
}

type First struct {
	pulumi.ResourceState
	PetName  pulumi.AnyOutput
	Password pulumi.AnyOutput
}

func NewFirst(
	ctx *pulumi.Context,
	name string,
	args *FirstArgs,
	opts ...pulumi.ResourceOption,
) (*First, error) {
	var componentResource First
	err := ctx.RegisterComponentResource("components:index:First", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	randomPet, err := random.NewRandomPet(ctx, fmt.Sprintf("%s-randomPet", name), nil, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	randomPassword, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-randomPassword", name), &random.RandomPasswordArgs{
		Length: args.PasswordLength,
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{
		"petName":  randomPet.ID(),
		"password": randomPassword.Result,
	})
	if err != nil {
		return nil, err
	}
	componentResource.PetName = randomPet
	componentResource.Password = randomPassword.Result
	return &componentResource, nil
}
