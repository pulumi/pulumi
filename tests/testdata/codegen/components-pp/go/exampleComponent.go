package main

import (
	"fmt"

	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type DeploymentZonesArgs struct {
	Zone pulumi.StringInput
}

type GithubAppArgs struct {
	Id            pulumi.StringInput
	KeyBase64     pulumi.StringInput
	WebhookSecret pulumi.StringInput
}

type ServersArgs struct {
	Name pulumi.StringInput
}

type ExampleComponentArgs struct {
	Input           pulumi.StringInput
	CidrBlocks      map[string]pulumi.StringInput
	GithubApp       *GithubAppArgs
	Servers         []*ServersArgs
	DeploymentZones map[string]*DeploymentZonesArgs
	IpAddress       []pulumi.IntInput
}

type ExampleComponent struct {
	pulumi.ResourceState
	Result pulumi.AnyOutput
}

func NewExampleComponent(
	ctx *pulumi.Context,
	name string,
	args *ExampleComponentArgs,
	opts ...pulumi.ResourceOption,
) (*ExampleComponent, error) {
	var componentResource ExampleComponent
	err := ctx.RegisterComponentResource("components:index:ExampleComponent", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	password, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-password", name), &random.RandomPasswordArgs{
		Length:          pulumi.Int(16),
		Special:         true,
		OverrideSpecial: args.Input,
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	_, err = random.NewRandomPassword(ctx, fmt.Sprintf("%s-githubPassword", name), &random.RandomPasswordArgs{
		Length:          pulumi.Int(16),
		Special:         true,
		OverrideSpecial: args.GithubApp.WebhookSecret,
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	// Example of iterating a list of objects
	var serverPasswords []*random.RandomPassword
	for index := 0; index < len(args.Servers); index++ {
		key0 := index
		val0 := index
		__res, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-serverPasswords-%v", name, key0), &random.RandomPasswordArgs{
			Length:          pulumi.Int(16),
			Special:         true,
			OverrideSpecial: pulumi.String(args.Servers[val0].Name),
		}, pulumi.Parent(&componentResource))
		if err != nil {
			return nil, err
		}
		serverPasswords = append(serverPasswords, __res)
	}
	// Example of iterating a map of objects
	var zonePasswords []*random.RandomPassword
	for key0, val0 := range args.DeploymentZones {
		__res, err := random.NewRandomPassword(ctx, fmt.Sprintf("%s-zonePasswords-%v", name, key0), &random.RandomPasswordArgs{
			Length:          pulumi.Int(16),
			Special:         true,
			OverrideSpecial: pulumi.String(val0),
		}, pulumi.Parent(&componentResource))
		if err != nil {
			return nil, err
		}
		zonePasswords = append(zonePasswords, __res)
	}
	_, err = NewSimpleComponent(ctx, fmt.Sprintf("%s-simpleComponent", name), nil, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{
		"result": password.Result,
	})
	if err != nil {
		return nil, err
	}
	componentResource.Result = password.Result
	return &componentResource, nil
}
