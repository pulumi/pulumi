// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

type Component struct {
	pulumi.ResourceState
}

type ComponentNested struct {
	Value string `pulumi:"value"`
}

type ComponentNestedInput interface {
	pulumi.Input

	ToComponentNestedOutput() ComponentNestedOutput
	ToComponentNestedOutputWithContext(context.Context) ComponentNestedOutput
}

type ComponentNestedArgs struct {
	Value pulumi.StringInput `pulumi:"value"`
}

func (ComponentNestedArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*ComponentNested)(nil)).Elem()
}

func (i ComponentNestedArgs) ToComponentNestedOutput() ComponentNestedOutput {
	return i.ToComponentNestedOutputWithContext(context.Background())
}

func (i ComponentNestedArgs) ToComponentNestedOutputWithContext(ctx context.Context) ComponentNestedOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentNestedOutput)
}

type ComponentNestedOutput struct{ *pulumi.OutputState }

func (ComponentNestedOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ComponentNested)(nil)).Elem()
}

func (o ComponentNestedOutput) ToComponentNestedOutput() ComponentNestedOutput {
	return o
}

func (o ComponentNestedOutput) ToComponentNestedOutputWithContext(ctx context.Context) ComponentNestedOutput {
	return o
}

type ComponentArgs struct {
	Message pulumi.StringInput   `pulumi:"message"`
	Nested  ComponentNestedInput `pulumi:"nested"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs,
	opts ...pulumi.ResourceOption) (*Component, error) {
	if args == nil {
		return nil, errors.New("args is required")
	}

	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	args.Message.ToStringOutput().ApplyT(func(val string) string {
		panic("should not run (message)")
	})
	args.Nested.ToComponentNestedOutput().ApplyT(func(val ComponentNested) string {
		panic("should not run (nested)")
	})

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

func init() {
	pulumi.RegisterOutputType(ComponentNestedOutput{})
}
