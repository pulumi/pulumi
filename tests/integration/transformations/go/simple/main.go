package main

import (
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

// SimpleResource is a resource with two outputs.
type SimpleResource struct {
	pulumi.ResourceState

	Output  string
	Output2 string
}

// SimpleComponent is a resource with a child.
type SimpleComponent struct {
	pulumi.ResourceState

	Child SimpleResource
}

// NewSimpleResource creates a new SimpleResource.
func NewSimpleResource(ctx *pulumi.Context, name string, props pulumi.Input, opts ...pulumi.ResourceOption) (*SimpleResource, error) {
	// inputs := &pulumi.Map{"outputs": nil, "output2": nil}

	res := &SimpleResource{}
	err := ctx.RegisterResource("pulumi-go:dynamic:Resource", name, nil, res, opts...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// NewSimpleComponent creates a new SimpleComponent with a child.
func NewSimpleComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*SimpleComponent, error) {
	comp := &SimpleComponent{}

	err := ctx.RegisterComponentResource("pulumi-go:dynamic:Component", name, comp, opts...)
	if err != nil {
		return nil, err
	}

	parentOpt := pulumi.Parent(comp)
	child, err := NewSimpleResource(ctx, name+"-child", nil, parentOpt)
	if err != nil {
		return nil, err
	}
	comp.Child = *child

	return comp, nil
}

// NewAlias returns an array with a single Alias.
func NewAlias(name string) []pulumi.Alias {
	var nilInput pulumi.StringInput
	aliasURN := pulumi.CreateURN(
		pulumi.StringInput(pulumi.String(name)),
		pulumi.StringInput(pulumi.String("my:module:SimpleResource")),
		nilInput,
		pulumi.StringInput(pulumi.String("project")),
		pulumi.StringInput(pulumi.String("stack")))
	alias := &pulumi.Alias{
		URN: aliasURN,
	}

	return []pulumi.Alias{*alias}
}

// res1Transformation adds an alias to the resourceOptions.
func res1Transformation(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
	opts := args.Opts
	opts.Aliases = NewAlias(args.Name)

	return &pulumi.ResourceTransformationResult{
		Props: args.Props,
		Opts:  opts,
	}
}

// res2Transformation adds an alias to a component's child.
func res2Transformation(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
	if args.Type == "pulumi-go:dynamic:Resource" {
		opts := args.Opts
		opts.Aliases = NewAlias(args.Name)

		props := args.Props
		pulumi.All(props).ApplyT

		return &pulumi.ResourceTransformationResult{
			Props: props,
			Opts:  opts,
		}
	}

	return nil
}

func res4Transformation1(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
	if args.Type == "pulumi-go:dynamic:Resource" {
		return &pulumi.ResourceTransformationResult{
			Props: args.Props,
			Opts:  args.Opts,
		}
	}
	return nil
}
func res4Transformation2(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
	if args.Type == "pulumi-go:dynamic:Resource" {
		return &pulumi.ResourceTransformationResult{
			Props: args.Props,
			Opts:  args.Opts,
		}
	}
	return nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Scenario #1 - apply a transformation to a CustomResource
		transformationOpt := pulumi.Transformations([]pulumi.ResourceTransformation{res1Transformation})
		_, err := NewSimpleResource(ctx, "res1", nil, transformationOpt)
		if err != nil {
			return err
		}

		// Scenario #2 - apply a transformation to a Component to transform it's children
		transformationOpt = pulumi.Transformations([]pulumi.ResourceTransformation{res2Transformation})
		_, err = NewSimpleComponent(ctx, "res2", transformationOpt)
		if err != nil {
			return err
		}

		return nil
	})
}
