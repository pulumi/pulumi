// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

// Exposes the Random resource from the testprovider.

package main

import (
	"errors"
	"reflect"

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
	var resource Random
	err := ctx.RegisterResource("testprovider:index:Random", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func (r *Random) RandomInvoke(ctx *pulumi.Context, args map[string]interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := ctx.Invoke("testprovider:index:returnArgs", args, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type randomArgs struct {
	Length int    `pulumi:"length"`
	Prefix string `pulumi:"prefix"`
}

type RandomArgs struct {
	Length pulumi.IntInput
	Prefix pulumi.StringInput
}

func (RandomArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*randomArgs)(nil)).Elem()
}

type Component struct {
	pulumi.ResourceState

	Length  pulumi.IntOutput    `pulumi:"length"`
	ChildID pulumi.StringOutput `pulumi:"childId"`
}

func NewComponent(ctx *pulumi.Context,
	name string, args *ComponentArgs, opts ...pulumi.ResourceOption,
) (*Random, error) {
	if args == nil || args.Length == nil {
		return nil, errors.New("missing required argument 'Length'")
	}
	var resource Random
	err := ctx.RegisterRemoteComponentResource("testprovider:index:Component", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type componentArgs struct {
	Length int `pulumi:"length"`
}

type ComponentArgs struct {
	Length pulumi.IntInput
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

type Provider struct {
	pulumi.ProviderResourceState
}

func NewProvider(ctx *pulumi.Context,
	name string, opts ...pulumi.ResourceOption,
) (*Provider, error) {
	var resource Provider
	err := ctx.RegisterResource("pulumi:providers:testprovider", name, nil, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}
