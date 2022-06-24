// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

type Component struct {
	pulumi.ResourceState
}

func NewComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Component, error) {
	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{}); err != nil {
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

			component, err := NewComponent(ctx, name, options)
			if err != nil {
				return nil, fmt.Errorf("creating component: %w", err)
			}

			return pulumiprovider.NewConstructResult(component)
		},
		Call: func(ctx *pulumi.Context, tok string, args pulumiprovider.CallArgs) (*pulumiprovider.CallResult, error) {
			if tok != "testcomponent:index:Component/getMessage" {
				return nil, fmt.Errorf("unknown method %s", tok)
			}

			return &pulumiprovider.CallResult{
				Failures: []pulumiprovider.CallFailure{
					{
						Property: "the failure property",
						Reason:   "the failure reason",
					},
				},
			}, nil
		},
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
