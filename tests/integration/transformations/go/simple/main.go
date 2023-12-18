// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
)

type MyComponent struct {
	pulumi.ResourceState
	Child *Random
}

func NewMyComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*MyComponent, error) {
	component := &MyComponent{}
	err := ctx.RegisterResource("my:component:MyComponent", name, nil, component, opts...)
	if err != nil {
		return nil, err
	}

	child, err := NewRandom(ctx, name+"-child", &RandomArgs{
		Length: pulumi.Int(5),
	}, pulumi.Parent(component), pulumi.AdditionalSecretOutputs([]string{"length"}))
	if err != nil {
		return nil, err
	}

	component.Child = child
	return component, nil
}

type MyOtherComponent struct {
	pulumi.ResourceState
	Child1 *Random
	Child2 *Random
}

func NewMyOtherComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*MyOtherComponent, error) {
	component := &MyOtherComponent{}
	err := ctx.RegisterResource("my:component:MyOtherComponent", name, nil, component, opts...)
	if err != nil {
		return nil, err
	}

	child1, err := NewRandom(ctx, name+"-child1", &RandomArgs{
		Length: pulumi.Int(5),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	child2, err := NewRandom(ctx, name+"-child2", &RandomArgs{
		Length: pulumi.Int(5),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	component.Child1 = child1
	component.Child2 = child2
	return component, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Scenario #1 - apply a transformation to a CustomResource
		_, err := NewRandom(ctx, "res1", &RandomArgs{Length: pulumi.Int(5)}, pulumi.Transformations([]pulumi.ResourceTransformation{
			func(rta *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				fmt.Printf("res1 transformation")
				return &pulumi.ResourceTransformationResult{
					Props: rta.Props,
					Opts:  append(rta.Opts, pulumi.AdditionalSecretOutputs([]string{"result"})),
				}
			},
		}))
		if err != nil {
			return err
		}

		// Scenario #2 - apply a transformation to a Component to transform it's children
		_, err = NewMyComponent(ctx, "res2", pulumi.Transformations([]pulumi.ResourceTransformation{
			func(rta *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				fmt.Printf("res2 transformation")
				if rta.Type == "testprovider:index:Random" {
					return &pulumi.ResourceTransformationResult{
						Props: rta.Props,
						Opts:  append(rta.Opts, pulumi.AdditionalSecretOutputs([]string{"result"})),
					}
				}
				return nil
			},
		}))
		if err != nil {
			return err
		}

		// Scenario #3 - apply a transformation to the Stack to transform all (future) resources in the stack
		err = ctx.RegisterStackTransformation(func(rta *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
			fmt.Printf("stack transformation")
			if rta.Type == "testprovider:index:Random" {
				var props *RandomArgs
				if rta.Props == nil {
					props = &RandomArgs{}
				} else {
					props = rta.Props.(*RandomArgs)
				}
				props.Prefix = pulumi.String("stackDefault")

				return &pulumi.ResourceTransformationResult{
					Props: props,
					Opts:  append(rta.Opts, pulumi.AdditionalSecretOutputs([]string{"result"})),
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		_, err = NewRandom(ctx, "res3", &RandomArgs{
			Length: pulumi.Int(5),
		})
		if err != nil {
			return err
		}

		// Scenario #4 - transformations are applied in order of decreasing specificity
		// 1. (not in this example) Child transformation
		// 2. First parent transformation
		// 3. Second parent transformation
		// 4. Stack transformation
		_, err = NewMyComponent(ctx, "res4", pulumi.Transformations([]pulumi.ResourceTransformation{
			func(rta *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				fmt.Printf("res4 transformation")
				if rta.Type == "testprovider:index:Random" {
					props := rta.Props.(*RandomArgs)
					props.Prefix = pulumi.String("default1")

					return &pulumi.ResourceTransformationResult{
						Props: props,
						Opts:  rta.Opts,
					}
				}
				return nil
			},
			func(rta *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				fmt.Printf("res4 transformation 2")
				if rta.Type == "testprovider:index:Random" {
					props := rta.Props.(*RandomArgs)
					props.Prefix = pulumi.String("default2")

					return &pulumi.ResourceTransformationResult{
						Props: props,
						Opts:  rta.Opts,
					}
				}
				return nil
			},
		}))
		if err != nil {
			return err
		}

		// Scenario #5 - cross-resource transformations that inject dependencies on one resource into another.

		// Create a promise that wil be resolved once we find child2.  This is needed because we do not
		// know what order we will see the resource registrations of child1 and child2.
		var child2Found promise.CompletionSource[*Random]
		// Return a transformation which will rewrite child1 to depend on the promise for child2, and will
		// resolve that promise when it finds child2.
		transformChild1DependsOnChild2 := func(rta *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
			if strings.HasSuffix(rta.Name, "-child2") {
				// Resolve the child2 promise with the child2 resource.
				child2Found.MustFulfill(rta.Resource.(*Random))
				return nil
			} else if strings.HasSuffix(rta.Name, "-child1") {
				props := rta.Props.(*RandomArgs)

				// Overwrite the `prefix` to child2 with a dependency on the `length` from child1.
				child2Result := pulumix.Flatten[string, pulumix.Output[string]](
					pulumix.ApplyErr[int, pulumix.Output[string]](
						props.Length.ToIntOutput().ToOutput(ctx.Context()),
						func(input int) (pulumix.Output[string], error) {
							var none pulumix.Output[string]

							if input != 5 {
								// Not strictly necessary - but shows we can confirm invariants we expect to be
								// true.
								return none, fmt.Errorf("unexpected input value")
							}

							args, err := child2Found.Promise().Result(ctx.Context())
							if err != nil {
								return none, err
							}

							return args.Result.ToOutput(ctx.Context()), nil
						}))

				// Finally - overwrite the input of child2.
				props.Prefix = child2Result.Untyped().(pulumi.StringInput)

				return &pulumi.ResourceTransformationResult{
					Props: props,
					Opts:  rta.Opts,
				}
			}
			return nil
		}

		_, err = NewMyOtherComponent(ctx, "res5", pulumi.Transformations([]pulumi.ResourceTransformation{transformChild1DependsOnChild2}))
		if err != nil {
			return err
		}

		return nil
	})
}
