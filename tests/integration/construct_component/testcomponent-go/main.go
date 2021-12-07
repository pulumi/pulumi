// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

type Component struct {
	pulumi.ResourceState

	Echo    pulumi.Input        `pulumi:"echo"`
	ChildID pulumi.IDOutput     `pulumi:"childId"`
	Secret  pulumi.StringOutput `pulumi:"secret"`
}

type ComponentArgs struct {
	Echo pulumi.Input `pulumi:"echo"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs,
	opts ...pulumi.ResourceOption) (*Component, error) {
	if args == nil {
		return nil, errors.New("args is required")
	}

	secretKey := "secret"
	fullSecretKey := fmt.Sprintf("%s:%s", ctx.Project(), secretKey)
	if !ctx.IsConfigSecret(fullSecretKey) {
		return nil, fmt.Errorf("expected configuration key to be secret: %s", fullSecretKey)
	}

	conf := config.New(ctx, "")
	secret := conf.RequireSecret(secretKey)

	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	res, err := NewEcho(ctx, fmt.Sprintf("child-%s", name), &EchoArgs{Echo: args.Echo}, pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	component.Echo = args.Echo
	component.ChildID = res.ID()
	component.Secret = secret

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"secret":  component.Secret,
		"echo":    component.Echo,
		"childId": component.ChildID,
	}); err != nil {
		return nil, err
	}

	return component, nil
}

const providerName = "testcomponent"
const version = "0.0.1"

func main() {
	if err := provider.MainWithOptions(provider.Options{
		Name:    providerName,
		Version: version,
		Construct: func(ctx *pulumi.Context, typ, name string, inputs pulumiprovider.ConstructInputs,
			options pulumi.ResourceOption) (*pulumiprovider.ConstructResult, error) {

			if typ != "testcomponent:index:Component" {
				return nil, fmt.Errorf("unknown resource type %s", typ)
			}

			args := &ComponentArgs{}
			if err := inputs.CopyTo(args); err != nil {
				return nil, fmt.Errorf("setting args: %w", err)
			}

			component, err := NewComponent(ctx, name, args, options)
			if err != nil {
				return nil, fmt.Errorf("creating component: %w", err)
			}

			return pulumiprovider.NewConstructResult(component)
		},
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
