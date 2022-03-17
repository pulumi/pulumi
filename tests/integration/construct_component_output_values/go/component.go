// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Component struct {
	pulumi.ResourceState
}

// NewComponent registers a new resource with the given unique name, arguments, and options.
func NewComponent(ctx *pulumi.Context,
	name string, args *ComponentArgs, opts ...pulumi.ResourceOption) (*Component, error) {
	if args == nil {
		args = &ComponentArgs{}
	}

	var resource Component
	err := ctx.RegisterRemoteComponentResource("testcomponent:index:Component", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type componentArgs struct {
	Bar *Bar `pulumi:"bar"`
	Foo *Foo `pulumi:"foo"`
}

// The set of arguments for constructing a Component resource.
type ComponentArgs struct {
	Bar BarPtrInput
	Foo *FooArgs
}

func (ComponentArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*componentArgs)(nil)).Elem()
}

type ComponentInput interface {
	pulumi.Input

	ToComponentOutput() ComponentOutput
	ToComponentOutputWithContext(ctx context.Context) ComponentOutput
}

func (*Component) ElementType() reflect.Type {
	return reflect.TypeOf((*Component)(nil))
}

func (i *Component) ToComponentOutput() ComponentOutput {
	return i.ToComponentOutputWithContext(context.Background())
}

func (i *Component) ToComponentOutputWithContext(ctx context.Context) ComponentOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentOutput)
}

func (i *Component) ToComponentPtrOutput() ComponentPtrOutput {
	return i.ToComponentPtrOutputWithContext(context.Background())
}

func (i *Component) ToComponentPtrOutputWithContext(ctx context.Context) ComponentPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentPtrOutput)
}

type ComponentPtrInput interface {
	pulumi.Input

	ToComponentPtrOutput() ComponentPtrOutput
	ToComponentPtrOutputWithContext(ctx context.Context) ComponentPtrOutput
}

type componentPtrType ComponentArgs

func (*componentPtrType) ElementType() reflect.Type {
	return reflect.TypeOf((**Component)(nil))
}

func (i *componentPtrType) ToComponentPtrOutput() ComponentPtrOutput {
	return i.ToComponentPtrOutputWithContext(context.Background())
}

func (i *componentPtrType) ToComponentPtrOutputWithContext(ctx context.Context) ComponentPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentPtrOutput)
}

// ComponentArrayInput is an input type that accepts ComponentArray and ComponentArrayOutput values.
// You can construct a concrete instance of `ComponentArrayInput` via:
//
//          ComponentArray{ ComponentArgs{...} }
type ComponentArrayInput interface {
	pulumi.Input

	ToComponentArrayOutput() ComponentArrayOutput
	ToComponentArrayOutputWithContext(context.Context) ComponentArrayOutput
}

type ComponentArray []ComponentInput

func (ComponentArray) ElementType() reflect.Type {
	return reflect.TypeOf((*[]*Component)(nil)).Elem()
}

func (i ComponentArray) ToComponentArrayOutput() ComponentArrayOutput {
	return i.ToComponentArrayOutputWithContext(context.Background())
}

func (i ComponentArray) ToComponentArrayOutputWithContext(ctx context.Context) ComponentArrayOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentArrayOutput)
}

// ComponentMapInput is an input type that accepts ComponentMap and ComponentMapOutput values.
// You can construct a concrete instance of `ComponentMapInput` via:
//
//          ComponentMap{ "key": ComponentArgs{...} }
type ComponentMapInput interface {
	pulumi.Input

	ToComponentMapOutput() ComponentMapOutput
	ToComponentMapOutputWithContext(context.Context) ComponentMapOutput
}

type ComponentMap map[string]ComponentInput

func (ComponentMap) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]*Component)(nil)).Elem()
}

func (i ComponentMap) ToComponentMapOutput() ComponentMapOutput {
	return i.ToComponentMapOutputWithContext(context.Background())
}

func (i ComponentMap) ToComponentMapOutputWithContext(ctx context.Context) ComponentMapOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentMapOutput)
}

type ComponentOutput struct{ *pulumi.OutputState }

func (ComponentOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Component)(nil))
}

func (o ComponentOutput) ToComponentOutput() ComponentOutput {
	return o
}

func (o ComponentOutput) ToComponentOutputWithContext(ctx context.Context) ComponentOutput {
	return o
}

func (o ComponentOutput) ToComponentPtrOutput() ComponentPtrOutput {
	return o.ToComponentPtrOutputWithContext(context.Background())
}

func (o ComponentOutput) ToComponentPtrOutputWithContext(ctx context.Context) ComponentPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v Component) *Component {
		return &v
	}).(ComponentPtrOutput)
}

type ComponentPtrOutput struct{ *pulumi.OutputState }

func (ComponentPtrOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Component)(nil))
}

func (o ComponentPtrOutput) ToComponentPtrOutput() ComponentPtrOutput {
	return o
}

func (o ComponentPtrOutput) ToComponentPtrOutputWithContext(ctx context.Context) ComponentPtrOutput {
	return o
}

func (o ComponentPtrOutput) Elem() ComponentOutput {
	return o.ApplyT(func(v *Component) Component {
		if v != nil {
			return *v
		}
		var ret Component
		return ret
	}).(ComponentOutput)
}

type ComponentArrayOutput struct{ *pulumi.OutputState }

func (ComponentArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]Component)(nil))
}

func (o ComponentArrayOutput) ToComponentArrayOutput() ComponentArrayOutput {
	return o
}

func (o ComponentArrayOutput) ToComponentArrayOutputWithContext(ctx context.Context) ComponentArrayOutput {
	return o
}

func (o ComponentArrayOutput) Index(i pulumi.IntInput) ComponentOutput {
	return pulumi.All(o, i).ApplyT(func(vs []interface{}) Component {
		return vs[0].([]Component)[vs[1].(int)]
	}).(ComponentOutput)
}

type ComponentMapOutput struct{ *pulumi.OutputState }

func (ComponentMapOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*map[string]Component)(nil))
}

func (o ComponentMapOutput) ToComponentMapOutput() ComponentMapOutput {
	return o
}

func (o ComponentMapOutput) ToComponentMapOutputWithContext(ctx context.Context) ComponentMapOutput {
	return o
}

func (o ComponentMapOutput) MapIndex(k pulumi.StringInput) ComponentOutput {
	return pulumi.All(o, k).ApplyT(func(vs []interface{}) Component {
		return vs[0].(map[string]Component)[vs[1].(string)]
	}).(ComponentOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*ComponentInput)(nil)).Elem(), &Component{})
	pulumi.RegisterInputType(reflect.TypeOf((*ComponentPtrInput)(nil)).Elem(), &Component{})
	pulumi.RegisterInputType(reflect.TypeOf((*ComponentArrayInput)(nil)).Elem(), ComponentArray{})
	pulumi.RegisterInputType(reflect.TypeOf((*ComponentMapInput)(nil)).Elem(), ComponentMap{})
	pulumi.RegisterOutputType(ComponentOutput{})
	pulumi.RegisterOutputType(ComponentPtrOutput{})
	pulumi.RegisterOutputType(ComponentArrayOutput{})
	pulumi.RegisterOutputType(ComponentMapOutput{})
}
