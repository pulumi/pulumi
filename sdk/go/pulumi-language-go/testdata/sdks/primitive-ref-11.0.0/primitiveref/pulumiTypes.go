// Code generated by pulumi-language-go DO NOT EDIT.
// *** WARNING: Do not edit by hand unless you're certain you know what you are doing! ***

package primitiveref

import (
	"context"
	"reflect"

	"example.com/pulumi-primitive-ref/sdk/go/v11/primitiveref/internal"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var _ = internal.GetEnvOrDefault

type Data struct {
	BoolArray []bool            `pulumi:"boolArray"`
	Boolean   bool              `pulumi:"boolean"`
	Float     float64           `pulumi:"float"`
	Integer   int               `pulumi:"integer"`
	String    string            `pulumi:"string"`
	StringMap map[string]string `pulumi:"stringMap"`
}

// DataInput is an input type that accepts DataArgs and DataOutput values.
// You can construct a concrete instance of `DataInput` via:
//
//	DataArgs{...}
type DataInput interface {
	pulumi.Input

	ToDataOutput() DataOutput
	ToDataOutputWithContext(context.Context) DataOutput
}

type DataArgs struct {
	BoolArray pulumi.BoolArrayInput `pulumi:"boolArray"`
	Boolean   pulumi.BoolInput      `pulumi:"boolean"`
	Float     pulumi.Float64Input   `pulumi:"float"`
	Integer   pulumi.IntInput       `pulumi:"integer"`
	String    pulumi.StringInput    `pulumi:"string"`
	StringMap pulumi.StringMapInput `pulumi:"stringMap"`
}

func (DataArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Data)(nil)).Elem()
}

func (i DataArgs) ToDataOutput() DataOutput {
	return i.ToDataOutputWithContext(context.Background())
}

func (i DataArgs) ToDataOutputWithContext(ctx context.Context) DataOutput {
	return pulumi.ToOutputWithContext(ctx, i).(DataOutput)
}

type DataOutput struct{ *pulumi.OutputState }

func (DataOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*Data)(nil)).Elem()
}

func (o DataOutput) ToDataOutput() DataOutput {
	return o
}

func (o DataOutput) ToDataOutputWithContext(ctx context.Context) DataOutput {
	return o
}

func (o DataOutput) BoolArray() pulumi.BoolArrayOutput {
	return o.ApplyT(func(v Data) []bool { return v.BoolArray }).(pulumi.BoolArrayOutput)
}

func (o DataOutput) Boolean() pulumi.BoolOutput {
	return o.ApplyT(func(v Data) bool { return v.Boolean }).(pulumi.BoolOutput)
}

func (o DataOutput) Float() pulumi.Float64Output {
	return o.ApplyT(func(v Data) float64 { return v.Float }).(pulumi.Float64Output)
}

func (o DataOutput) Integer() pulumi.IntOutput {
	return o.ApplyT(func(v Data) int { return v.Integer }).(pulumi.IntOutput)
}

func (o DataOutput) String() pulumi.StringOutput {
	return o.ApplyT(func(v Data) string { return v.String }).(pulumi.StringOutput)
}

func (o DataOutput) StringMap() pulumi.StringMapOutput {
	return o.ApplyT(func(v Data) map[string]string { return v.StringMap }).(pulumi.StringMapOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*DataInput)(nil)).Elem(), DataArgs{})
	pulumi.RegisterOutputType(DataOutput{})
}
