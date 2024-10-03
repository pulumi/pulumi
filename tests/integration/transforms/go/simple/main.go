// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
		// Scenario #1 - apply a transform to a CustomResource
		_, err := NewRandom(ctx, "res1", &RandomArgs{Length: pulumi.Int(5)}, pulumi.Transforms([]pulumi.ResourceTransform{
			func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
				fmt.Printf("res1 transform\n")
				rta.Opts.AdditionalSecretOutputs = append(rta.Opts.AdditionalSecretOutputs, "result")
				return &pulumi.ResourceTransformResult{
					Props: rta.Props,
					Opts:  rta.Opts,
				}
			},
		}))
		if err != nil {
			return err
		}

		// Scenario #2 - apply a transform to a Component to transform it's children
		_, err = NewMyComponent(ctx, "res2", pulumi.Transforms([]pulumi.ResourceTransform{
			func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
				fmt.Printf("res2 transform\n")
				if rta.Type == "testprovider:index:Random" {
					rta.Opts.AdditionalSecretOutputs = append(rta.Opts.AdditionalSecretOutputs, "result")
					return &pulumi.ResourceTransformResult{
						Props: rta.Props,
						Opts:  rta.Opts,
					}
				}
				return nil
			},
		}))
		if err != nil {
			return err
		}

		// Scenario #3 - apply a transform to the Stack to transform all (future) resources in the stack
		err = ctx.RegisterStackTransform(func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
			fmt.Printf("stack transform\n")
			fmt.Printf("%v %v\n", rta.Type, rta.Props)
			if rta.Type == "testprovider:index:Random" {
				rta.Props["prefix"] = pulumi.String("stackDefault")
				rta.Opts.AdditionalSecretOutputs = append(rta.Opts.AdditionalSecretOutputs, "result")

				return &pulumi.ResourceTransformResult{
					Props: rta.Props,
					Opts:  rta.Opts,
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		_, err = NewRandom(ctx, "res3", &RandomArgs{
			Length: pulumi.ToSecret(pulumi.Int(5)).(pulumi.IntOutput),
		})
		if err != nil {
			return err
		}

		// Scenario #4 - transforms are applied in order of decreasing specificity
		// 1. (not in this example) Child transform
		// 2. First parent transform
		// 3. Second parent transform
		// 4. Stack transform
		_, err = NewMyComponent(ctx, "res4", pulumi.Transforms([]pulumi.ResourceTransform{
			func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
				fmt.Printf("res4 transform\n")
				if rta.Type == "testprovider:index:Random" {
					rta.Props["prefix"] = pulumi.String("default1")

					return &pulumi.ResourceTransformResult{
						Props: rta.Props,
						Opts:  rta.Opts,
					}
				}
				return nil
			},
			func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
				fmt.Printf("res4 transform 2\n")
				if rta.Type == "testprovider:index:Random" {
					rta.Props["prefix"] = pulumi.String("default2")

					return &pulumi.ResourceTransformResult{
						Props: rta.Props,
						Opts:  rta.Opts,
					}
				}
				return nil
			},
		}))
		if err != nil {
			return err
		}

		// Scenario #5 - mutate the properties of a resource
		_, err = NewRandom(ctx, "res5", &RandomArgs{Length: pulumi.Int(10)}, pulumi.Transforms([]pulumi.ResourceTransform{
			func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
				fmt.Printf("res5 transform\n")
				if rta.Type == "testprovider:index:Random" {
					length := rta.Props["length"].(pulumi.Float64)
					rta.Props["length"] = length * 2

					return &pulumi.ResourceTransformResult{
						Props: rta.Props,
						Opts:  rta.Opts,
					}
				}
				return nil
			},
		}))
		if err != nil {
			return err
		}

		// Scenario #6 - mutate the provider on a custom resource
		provider1, err := NewProvider(ctx, "provider1")
		if err != nil {
			return err
		}
		provider2, err := NewProvider(ctx, "provider2")
		if err != nil {
			return err
		}

		_, err = NewRandom(ctx, "res6", &RandomArgs{Length: pulumi.Int(10)},
			pulumi.Provider(provider1),
			pulumi.Transforms([]pulumi.ResourceTransform{
				func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
					fmt.Printf("res6 transform\n")
					rta.Opts.Provider = provider2
					return &pulumi.ResourceTransformResult{
						Props: rta.Props,
						Opts:  rta.Opts,
					}
				},
			}),
		)
		if err != nil {
			return err
		}

		// Scenario #7 - mutate the provider on a component resource
		_, err = NewComponent(ctx, "res7", &ComponentArgs{Length: pulumi.Int(10)},
			pulumi.Provider(provider1),
			pulumi.Transforms([]pulumi.ResourceTransform{
				func(_ context.Context, rta *pulumi.ResourceTransformArgs) *pulumi.ResourceTransformResult {
					fmt.Printf("res7 transform\n")
					rta.Opts.Provider = provider2
					return &pulumi.ResourceTransformResult{
						Props: rta.Props,
						Opts:  rta.Opts,
					}
				},
			}),
		)
		if err != nil {
			return err
		}

		// Scenario #8 - run an invoke and change args
		err = ctx.RegisterInvokeTransform(func(_ context.Context, ita *pulumi.InvokeTransformArgs) *pulumi.InvokeTransformResult {
			ita.Args["length"] = pulumi.Float64(11)
			return &pulumi.InvokeTransformResult{
				Args: ita.Args,
				Opts: ita.Opts,
			}
		})
		if err != nil {
			return err
		}
		res8, err := NewRandom(ctx, "res8", &RandomArgs{Length: pulumi.Int(10)})
		if err != nil {
			return err
		}
		args := map[string]interface{}{
			"length": 10,
			"prefix": "test",
		}

		result, err := res8.RandomInvoke(ctx, args)
		if err != nil {
			return err
		}
		length, _ := result["length"].(float64)
		if length != 11 {
			return fmt.Errorf("expected length to be 11, got %v", length)
		}
		if result["prefix"] != "test" {
			return fmt.Errorf("expected prefix to be test, got %v", result["prefix"])
		}

		return nil
	})
}
