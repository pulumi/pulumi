// Copyright 2016-2022, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"fmt"
	"reflect"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/internals"
)

type Child struct {
	pulumi.ResourceState

	Message pulumi.StringOutput `pulumi:"message"`
}

func NewChild(ctx *pulumi.Context, name string, message pulumi.StringInput,
	opts ...pulumi.ResourceOption) (*Child, error) {

	component := &Child{}
	if err := ctx.RegisterComponentResource("test:index:Child", name, component, opts...); err != nil {
		return nil, err
	}
	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"message": message,
	}); err != nil {
		return nil, err
	}
	return component, nil
}

type ChildInput interface {
	pulumi.Input
}

func (*Child) ElementType() reflect.Type {
	return reflect.TypeOf((**Child)(nil)).Elem()
}

type ChildOutput struct{ *pulumi.OutputState }

func (ChildOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Child)(nil)).Elem()
}

type Container struct {
	pulumi.ResourceState

	Child ChildOutput `pulumi:"child"`
}

func NewContainer(ctx *pulumi.Context, name string, child ChildInput,
	opts ...pulumi.ResourceOption) (*Container, error) {

	component := &Container{}
	if err := ctx.RegisterComponentResource("test:index:Container", name, component, opts...); err != nil {
		return nil, err
	}
	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"child": child,
	}); err != nil {
		return nil, err
	}
	return component, nil
}

type module struct {
	version semver.Version
}

func (m *module) Version() semver.Version {
	return m.version
}

func (m *module) Construct(ctx *pulumi.Context, name, typ, urn string) (r pulumi.Resource, err error) {
	switch typ {
	case "test:index:Child":
		r = &Child{}
	default:
		return nil, fmt.Errorf("unknown resource type: %s", typ)
	}
	err = ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))
	return
}

func main() {
	pulumi.RegisterOutputType(ChildOutput{})
	pulumi.RegisterResourceModule(
		"test",
		"index",
		&module{semver.MustParse("1.0.0")},
	)

	pulumi.Run(func(ctx *pulumi.Context) error {
		child, err := NewChild(ctx, "mychild", pulumi.String("hello world!"))
		if err != nil {
			return err
		}

		container, err := NewContainer(ctx, "mycontainer", child)
		if err != nil {
			return err
		}

		containerURNResult, err := internals.UnsafeAwaitOutput(ctx.Context(), container.URN())
		if err != nil {
			return err
		}
		containerURN := containerURNResult.Value.(pulumi.URN)

		roundTrippedContainer := &Container{}
		if err := ctx.RegisterComponentResource("test:index:Container", "mycontainer", roundTrippedContainer,
			pulumi.URN_(string(containerURN))); err != nil {
			return err
		}

		roundTrippedContainerChildResult, err := internals.UnsafeAwaitOutput(ctx.Context(), roundTrippedContainer.Child)
		if err != nil {
			return err
		}
		roundTrippedContainerChild := roundTrippedContainerChildResult.Value.(*Child)

		pulumi.All(child.URN(), roundTrippedContainerChild.URN(), roundTrippedContainerChild.Message).ApplyT(
			func(args []interface{}) (*string, error) {
				const expectedMessage = "hello world!"
				expectedURN := args[0].(pulumi.URN)
				actualURN := args[1].(pulumi.URN)
				actualMessage := args[2].(string)
				if expectedURN != actualURN {
					panic(fmt.Errorf("expected urn %q not equal to actual urn %q", expectedURN, actualMessage))
				}
				if expectedMessage != actualMessage {
					panic(fmt.Errorf(
						"expected message %q not equal to actual message %q", expectedMessage, actualMessage))
				}
				return nil, nil
			})

		return nil
	})
}
