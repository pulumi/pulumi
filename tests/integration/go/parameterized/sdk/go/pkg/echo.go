// Code generated by pulumi-language-go DO NOT EDIT.
// *** WARNING: Do not edit by hand unless you're certain you know what you are doing! ***

package pkg

import (
	"context"
	"reflect"

	"example.com/pulumi-pkg/sdk/go/pkg/internal"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// A test resource that echoes its input.
type Echo struct {
	pulumi.CustomResourceState

	// Input to echo.
	Echo pulumi.AnyOutput `pulumi:"echo"`
}

// NewEcho registers a new resource with the given unique name, arguments, and options.
func NewEcho(ctx *pulumi.Context,
	name string, args *EchoArgs, opts ...pulumi.ResourceOption) (*Echo, error) {
	if args == nil {
		args = &EchoArgs{}
	}

	opts = internal.PkgResourceDefaultOpts(opts)
	ref, err := internal.PkgGetPackageRef(ctx)
	if err != nil {
		return nil, err
	}
	var resource Echo
	err = ctx.RegisterPackageResource("pkg:index:Echo", name, args, &resource, ref, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

// GetEcho gets an existing Echo resource's state with the given name, ID, and optional
// state properties that are used to uniquely qualify the lookup (nil if not required).
func GetEcho(ctx *pulumi.Context,
	name string, id pulumi.IDInput, state *EchoState, opts ...pulumi.ResourceOption) (*Echo, error) {
	var resource Echo
	ref, err := internal.PkgGetPackageRef(ctx)
	if err != nil {
		return nil, err
	}
	err = ctx.ReadPackageResource("pkg:index:Echo", name, id, state, &resource, ref, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

// Input properties used for looking up and filtering Echo resources.
type echoState struct {
}

type EchoState struct {
}

func (EchoState) ElementType() reflect.Type {
	return reflect.TypeOf((*echoState)(nil)).Elem()
}

type echoArgs struct {
	// An echoed input.
	Echo interface{} `pulumi:"echo"`
}

// The set of arguments for constructing a Echo resource.
type EchoArgs struct {
	// An echoed input.
	Echo pulumi.Input
}

func (EchoArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*echoArgs)(nil)).Elem()
}

// A test call that echoes its input.
func (r *Echo) DoEchoMethod(ctx *pulumi.Context, args *EchoDoEchoMethodArgs) (EchoDoEchoMethodResultOutput, error) {
	ref, err := internal.PkgGetPackageRef(ctx)
	if err != nil {
		return EchoDoEchoMethodResultOutput{}, err
	}
	out, err := ctx.CallPackage("pkg:index:Echo/doEchoMethod", args, EchoDoEchoMethodResultOutput{}, r, ref)
	if err != nil {
		return EchoDoEchoMethodResultOutput{}, err
	}
	return out.(EchoDoEchoMethodResultOutput), nil
}

type echoDoEchoMethodArgs struct {
	Echo *string `pulumi:"echo"`
}

// The set of arguments for the DoEchoMethod method of the Echo resource.
type EchoDoEchoMethodArgs struct {
	Echo pulumi.StringPtrInput
}

func (EchoDoEchoMethodArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*echoDoEchoMethodArgs)(nil)).Elem()
}

type EchoDoEchoMethodResult struct {
	Echo *string `pulumi:"echo"`
}

type EchoDoEchoMethodResultOutput struct{ *pulumi.OutputState }

func (EchoDoEchoMethodResultOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*EchoDoEchoMethodResult)(nil)).Elem()
}

func (o EchoDoEchoMethodResultOutput) Echo() pulumi.StringPtrOutput {
	return o.ApplyT(func(v EchoDoEchoMethodResult) *string { return v.Echo }).(pulumi.StringPtrOutput)
}

type EchoInput interface {
	pulumi.Input

	ToEchoOutput() EchoOutput
	ToEchoOutputWithContext(ctx context.Context) EchoOutput
}

func (*Echo) ElementType() reflect.Type {
	return reflect.TypeOf((**Echo)(nil)).Elem()
}

func (i *Echo) ToEchoOutput() EchoOutput {
	return i.ToEchoOutputWithContext(context.Background())
}

func (i *Echo) ToEchoOutputWithContext(ctx context.Context) EchoOutput {
	return pulumi.ToOutputWithContext(ctx, i).(EchoOutput)
}

type EchoOutput struct{ *pulumi.OutputState }

func (EchoOutput) ElementType() reflect.Type {
	return reflect.TypeOf((**Echo)(nil)).Elem()
}

func (o EchoOutput) ToEchoOutput() EchoOutput {
	return o
}

func (o EchoOutput) ToEchoOutputWithContext(ctx context.Context) EchoOutput {
	return o
}

// Input to echo.
func (o EchoOutput) Echo() pulumi.AnyOutput {
	return o.ApplyT(func(v *Echo) pulumi.AnyOutput { return v.Echo }).(pulumi.AnyOutput)
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*EchoInput)(nil)).Elem(), &Echo{})
	pulumi.RegisterOutputType(EchoOutput{})
	pulumi.RegisterOutputType(EchoDoEchoMethodResultOutput{})
}
