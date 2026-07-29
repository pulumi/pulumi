package main

import (
	"example.com/pulumi-config/sdk/go/v9/config"
	"example.com/pulumi-multi-argument-invoke/sdk/go/v44/multiargumentinvoke"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type InvokeComponentArgs struct {
}

type InvokeComponent struct {
	pulumi.ResourceState
	Result pulumi.AnyOutput
}

func NewInvokeComponent(
	ctx *pulumi.Context,
	name string,
	args *InvokeComponentArgs,
	opts ...pulumi.ResourceOption,
) (*InvokeComponent, error) {
	var componentResource InvokeComponent
	err := ctx.RegisterComponentResource("components:index:InvokeComponent", name, &componentResource, opts...)
	if err != nil {
		return nil, err
	}
	// A multi-argument invoke passes its arguments positionally and omits the ones the program leaves
	// out, so parenting it must not displace the options bag into an argument slot.
	greeting := multiargumentinvoke.MultiArgumentInvokeOutput(ctx, pulumi.String("hello"), nil, pulumi.Parent(&componentResource))
	providerConfig := config.GetConfigOutput(ctx, config.GetConfigOutputArgs{
		Text: greeting.ApplyT(func(greeting multiargumentinvoke.MultiArgumentInvokeResult) (string, error) {
			return greeting.Result, nil
		}).(pulumi.StringOutput),
	}, pulumi.Parent(&componentResource))
	err = ctx.RegisterResourceOutputs(&componentResource, pulumi.Map{
		"result": providerConfig.ApplyT(func(providerConfig config.GetConfigResult) (string, error) {
			return providerConfig.Text, nil
		}).(pulumi.StringOutput),
	})
	if err != nil {
		return nil, err
	}
	componentResource.Result = pulumi.Any(providerConfig.ApplyT(func(providerConfig config.GetConfigResult) (string, error) {
		return providerConfig.Text, nil
	}).(pulumi.StringOutput))
	return &componentResource, nil
}
