package main

import "github.com/pulumi/pulumi/sdk/go/pulumi"

type SimpleResource struct {
	pulumi.ResourceState
}

func NewSimpleResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*SimpleResource, error) {
	res := &SimpleResource{}
	err := ctx.RegisterComponentResource("SimpleResource", name, res, opts...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func res1_transformation(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
	var nilInput pulumi.StringInput
	aliasURN := pulumi.CreateURN(
		pulumi.StringInput(pulumi.String("res1")),
		pulumi.StringInput(pulumi.String("my:module:SimpleResource")),
		nilInput,
		pulumi.StringInput(pulumi.String("project")),
		pulumi.StringInput(pulumi.String("stack")))
	alias := &pulumi.Alias{
		URN: aliasURN,
	}

	opts := args.Opts
	opts.Aliases = []pulumi.Alias{*alias}

	return &pulumi.ResourceTransformationResult{
		Props: args.Props,
		Opts:  opts,
	}
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		resource := &SimpleResource{}
		transformationOpt := pulumi.Transformations([]pulumi.ResourceTransformation{res1_transformation})
		err := ctx.RegisterComponentResource("pulumi-go:dynamic:Resource", "res1", resource, transformationOpt)
		if err != nil {
			return err
		}

		return nil
	})
}
