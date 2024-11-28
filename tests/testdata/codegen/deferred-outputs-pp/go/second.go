package main

import (
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type SecondArgs struct {
	PetName pulumi.StringInput
}

type Second struct {
	pulumi.ResourceState
	PasswordLength pulumi.AnyOutput
}

func NewSecond(
	ctx *pulumi.Context,
	name string,
	args *SecondArgs,
	opts ...pulumi.ResourceOption,
) (*Second, error) {
	var componentResource Second
	err := ctx.RegisterComponentResource("components:index:Second", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	_, err = random.NewRandomPet(ctx, fmt.Sprintf("%s-randomPet", name), &random.RandomPetArgs{
		Length: len(args.PetName),
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	password, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-password", name), &random.RandomPasswordArgs{
		Length:  pulumi.Int(16),
		Special: pulumi.Bool(true),
		Numeric: pulumi.Bool(false),
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{
		"passwordLength": password.Length,
	})
	if err != nil {
		return nil, err
	}
	componentResource.PasswordLength = password.Length
	return &componentResource, nil
}
