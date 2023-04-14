//go:build !all
// +build !all

package main

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		newComponent := func(name string, opts ...pulumi.ResourceOption) (*Component, error) {
			return NewComponent(ctx, name, &ComponentArgs{Echo: pulumi.String(name)}, opts...)
		}

		dep1, err := newComponent("Dep1")
		if err != nil {
			return err
		}
		dep2, err := newComponent("Dep2")
		if err != nil {
			return err
		}
		_, err = newComponent("DependsOn", pulumi.DependsOn([]pulumi.Resource{dep1, dep2}))
		if err != nil {
			return err
		}

		_, err = newComponent("Protect", pulumi.Protect(true))
		if err != nil {
			return err
		}

		_, err = newComponent("AdditionalSecretOutputs", pulumi.AdditionalSecretOutputs([]string{"foo"}))
		if err != nil {
			return err
		}

		_, err = newComponent("CustomTimeouts", pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: ("1m"),
			Update: ("2m"),
			Delete: ("3m"),
		}))
		if err != nil {
			return err
		}

		getDeletedWithMe, err := newComponent("getDeletedWithMe")
		if err != nil {
			return err
		}
		_, err = newComponent("DeletedWith", pulumi.DeletedWith(getDeletedWithMe))
		if err != nil {
			return err
		}

		_, err = newComponent("RetainOnDelete", pulumi.RetainOnDelete(true))
		if err != nil {
			return err
		}

		return nil
	})
}

type Component struct {
	pulumi.ResourceState

	Echo pulumi.StringOutput `pulumi:"echo"`
	Foo  pulumi.StringOutput `pulumi:"foo"`
	Bar  pulumi.StringOutput `pulumi:"bar"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs, opts ...pulumi.ResourceOption) (*Component, error) {
	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, args, &resource, opts...)
	return &resource, err
}

type ComponentArgs struct {
	Echo pulumi.StringInput
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

type componentArgs struct {
	Echo string `pulumi:"echo"`
}
