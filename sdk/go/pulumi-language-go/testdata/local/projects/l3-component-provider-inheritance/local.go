package main

import (
	"fmt"

	"example.com/pulumi-component/sdk/go/v13/component"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LocalArgs struct {
}

type Local struct {
	pulumi.ResourceState
	Result pulumi.AnyOutput
}

func NewLocal(
	ctx *pulumi.Context,
	name string,
	args *LocalArgs,
	opts ...pulumi.ResourceOption,
) (*Local, error) {
	var componentResource Local
	err := ctx.RegisterComponentResource("components:index:Local", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	// No provider options here: the providers map must be inherited from the
	// enclosing local component and flow through the remote component's
	// registration into its construct call.
	mlc, err := component.NewComponentForeignChild(ctx, fmt.Sprintf("%s-mlc", name), &component.ComponentForeignChildArgs{
		Value: pulumi.Bool(true),
	}, pulumi.Parent(&componentResource))
	if err != nil {
		return nil, err
	}
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{
		"result": mlc.Value,
	})
	if err != nil {
		return nil, err
	}
	componentResource.Result = pulumi.Any(mlc.Value)
	return &componentResource, nil
}
