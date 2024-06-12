// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

// Exposes the Random resource from the testprovider.

package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Random struct {
	pulumi.CustomResourceState

	Length pulumi.IntOutput    `pulumi:"length"`
	Result pulumi.StringOutput `pulumi:"result"`
}

func NewRandom(ctx *pulumi.Context,
	name string, args *RandomArgs, opts ...pulumi.ResourceOption,
) (*Random, error) {
	if args == nil || args.Length == nil {
		return nil, errors.New("missing required argument 'Length'")
	}
	if args == nil {
		args = &RandomArgs{}
	}
	var resource Random
	err := ctx.RegisterResource("testprovider:index:Random", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

// Input properties used for looking up and filtering Random resources.
type randomState struct {
}

type RandomState struct {
}

func (RandomState) ElementType() reflect.Type {
	return reflect.TypeOf((*randomState)(nil)).Elem()
}

type randomArgs struct {
	Length int `pulumi:"length"`
}

type RandomArgs struct {
	Length pulumi.IntInput
}

func (RandomArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*randomArgs)(nil)).Elem()
}

type RandomInput interface {
	pulumi.Input

	ToRandomOutput() RandomOutput
	ToRandomOutputWithContext(ctx context.Context) RandomOutput
}

func (*Random) ElementType() reflect.Type {
	return reflect.TypeOf((**Random)(nil)).Elem()
}

func (i *Random) ToRandomOutput() RandomOutput {
	return i.ToRandomOutputWithContext(context.Background())
}

func (i *Random) ToRandomOutputWithContext(ctx context.Context) RandomOutput {
	return pulumi.ToOutputWithContext(ctx, i).(RandomOutput)
}

type RandomOutput struct{ *pulumi.OutputState }

func (RandomOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Random)(nil)).Elem()
}

func (o RandomOutput) ToRandomOutput() RandomOutput {
	return o
}

func (o RandomOutput) ToRandomOutputWithContext(ctx context.Context) RandomOutput {
	return o
}

func (o RandomOutput) Result() pulumi.StringOutput {
	return o.ApplyT(func(v *Random) pulumi.StringOutput { return v.Result }).(pulumi.StringOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*RandomInput)(nil)).Elem(), &Random{})
	pulumi.RegisterOutputType(RandomOutput{})
}


type module struct {
	version semver.Version
}


func (m *module) Version() semver.Version {
	return m.version
}

func (m *module) Construct(ctx *pulumi.Context, name, typ, urn string) (r pulumi.Resource, err error) {
	switch typ {
	case "testprovider:index:Random":
		r = &Random{}
	default:
		return nil, fmt.Errorf("unknown resource type: %s", typ)
	}

	err = ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))
	return
}

func init() {
		version := semver.Version{Major: 1}
	pulumi.RegisterResourceModule(
		"testprovider",
		"index",
		&module{version},
	)
}
