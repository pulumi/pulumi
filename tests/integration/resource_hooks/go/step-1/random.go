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

type randomArgs struct {
	Length int `pulumi:"length"`
}

type RandomArgs struct {
	Length pulumi.IntInput
}

func (RandomArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*randomArgs)(nil)).Elem()
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
