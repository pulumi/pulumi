// Copyright 2016, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !all

package main

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Bar struct {
	Tags map[string]string `pulumi:"tags"`
}

// BarInput is an input type that accepts BarArgs and BarOutput values.
// You can construct a concrete instance of `BarInput` via:
//
//	BarArgs{...}
type BarInput interface {
	pulumi.Input

	ToBarOutput() BarOutput
	ToBarOutputWithContext(context.Context) BarOutput
}

type BarArgs struct {
	Tags pulumi.StringMapInput `pulumi:"tags"`
}

func (BarArgs) ElementType() reflect.Type {
	return reflect.TypeFor[Bar]()
}

func (i BarArgs) ToBarOutput() BarOutput {
	return i.ToBarOutputWithContext(context.Background())
}

func (i BarArgs) ToBarOutputWithContext(ctx context.Context) BarOutput {
	return pulumi.ToOutputWithContext(ctx, i).(BarOutput)
}

func (i BarArgs) ToBarPtrOutput() BarPtrOutput {
	return i.ToBarPtrOutputWithContext(context.Background())
}

func (i BarArgs) ToBarPtrOutputWithContext(ctx context.Context) BarPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(BarOutput).ToBarPtrOutputWithContext(ctx)
}

// BarPtrInput is an input type that accepts BarArgs, BarPtr and BarPtrOutput values.
// You can construct a concrete instance of `BarPtrInput` via:
//
//	        BarArgs{...}
//
//	or:
//
//	        nil
type BarPtrInput interface {
	pulumi.Input

	ToBarPtrOutput() BarPtrOutput
	ToBarPtrOutputWithContext(context.Context) BarPtrOutput
}

type BarOutput struct{ *pulumi.OutputState }

func (BarOutput) ElementType() reflect.Type {
	return reflect.TypeFor[Bar]()
}

func (o BarOutput) ToBarOutput() BarOutput {
	return o
}

func (o BarOutput) ToBarOutputWithContext(ctx context.Context) BarOutput {
	return o
}

func (o BarOutput) ToBarPtrOutput() BarPtrOutput {
	return o.ToBarPtrOutputWithContext(context.Background())
}

func (o BarOutput) ToBarPtrOutputWithContext(ctx context.Context) BarPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v Bar) *Bar {
		return &v
	}).(BarPtrOutput)
}

func (o BarOutput) Tags() pulumi.StringMapOutput {
	return o.ApplyT(func(v Bar) map[string]string { return v.Tags }).(pulumi.StringMapOutput)
}

type BarPtrOutput struct{ *pulumi.OutputState }

func (BarPtrOutput) ElementType() reflect.Type {
	return reflect.TypeFor[*Bar]()
}

func (o BarPtrOutput) ToBarPtrOutput() BarPtrOutput {
	return o
}

func (o BarPtrOutput) ToBarPtrOutputWithContext(ctx context.Context) BarPtrOutput {
	return o
}

func (o BarPtrOutput) Elem() BarOutput {
	return o.ApplyT(func(v *Bar) Bar {
		if v != nil {
			return *v
		}
		var ret Bar
		return ret
	}).(BarOutput)
}

func (o BarPtrOutput) Tags() pulumi.StringMapOutput {
	return o.ApplyT(func(v *Bar) map[string]string {
		if v == nil {
			return nil
		}
		return v.Tags
	}).(pulumi.StringMapOutput)
}

type Foo struct {
	Something *string `pulumi:"something"`
}

// FooInput is an input type that accepts FooArgs and FooOutput values.
// You can construct a concrete instance of `FooInput` via:
//
//	FooArgs{...}
type FooInput interface {
	pulumi.Input

	ToFooOutput() FooOutput
	ToFooOutputWithContext(context.Context) FooOutput
}

type FooArgs struct {
	Something pulumi.StringPtrInput `pulumi:"something"`
}

func (FooArgs) ElementType() reflect.Type {
	return reflect.TypeFor[Foo]()
}

func (i FooArgs) ToFooOutput() FooOutput {
	return i.ToFooOutputWithContext(context.Background())
}

func (i FooArgs) ToFooOutputWithContext(ctx context.Context) FooOutput {
	return pulumi.ToOutputWithContext(ctx, i).(FooOutput)
}

func (i FooArgs) ToFooPtrOutput() FooPtrOutput {
	return i.ToFooPtrOutputWithContext(context.Background())
}

func (i FooArgs) ToFooPtrOutputWithContext(ctx context.Context) FooPtrOutput {
	return pulumi.ToOutputWithContext(ctx, i).(FooOutput).ToFooPtrOutputWithContext(ctx)
}

// FooPtrInput is an input type that accepts FooArgs, FooPtr and FooPtrOutput values.
// You can construct a concrete instance of `FooPtrInput` via:
//
//	        FooArgs{...}
//
//	or:
//
//	        nil
type FooPtrInput interface {
	pulumi.Input

	ToFooPtrOutput() FooPtrOutput
	ToFooPtrOutputWithContext(context.Context) FooPtrOutput
}

type FooOutput struct{ *pulumi.OutputState }

func (FooOutput) ElementType() reflect.Type {
	return reflect.TypeFor[Foo]()
}

func (o FooOutput) ToFooOutput() FooOutput {
	return o
}

func (o FooOutput) ToFooOutputWithContext(ctx context.Context) FooOutput {
	return o
}

func (o FooOutput) ToFooPtrOutput() FooPtrOutput {
	return o.ToFooPtrOutputWithContext(context.Background())
}

func (o FooOutput) ToFooPtrOutputWithContext(ctx context.Context) FooPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v Foo) *Foo {
		return &v
	}).(FooPtrOutput)
}

func (o FooOutput) Something() pulumi.StringPtrOutput {
	return o.ApplyT(func(v Foo) *string { return v.Something }).(pulumi.StringPtrOutput)
}

type FooPtrOutput struct{ *pulumi.OutputState }

func (FooPtrOutput) ElementType() reflect.Type {
	return reflect.TypeFor[*Foo]()
}

func (o FooPtrOutput) ToFooPtrOutput() FooPtrOutput {
	return o
}

func (o FooPtrOutput) ToFooPtrOutputWithContext(ctx context.Context) FooPtrOutput {
	return o
}

func (o FooPtrOutput) Elem() FooOutput {
	return o.ApplyT(func(v *Foo) Foo {
		if v != nil {
			return *v
		}
		var ret Foo
		return ret
	}).(FooOutput)
}

func (o FooPtrOutput) Something() pulumi.StringPtrOutput {
	return o.ApplyT(func(v *Foo) *string {
		if v == nil {
			return nil
		}
		return v.Something
	}).(pulumi.StringPtrOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeFor[BarInput](), BarArgs{})
	pulumi.RegisterInputType(reflect.TypeFor[BarPtrInput](), BarArgs{})
	pulumi.RegisterInputType(reflect.TypeFor[FooInput](), FooArgs{})
	pulumi.RegisterInputType(reflect.TypeFor[FooPtrInput](), FooArgs{})
	pulumi.RegisterOutputType(BarOutput{})
	pulumi.RegisterOutputType(BarPtrOutput{})
	pulumi.RegisterOutputType(FooOutput{})
	pulumi.RegisterOutputType(FooPtrOutput{})
}
