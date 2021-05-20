package codegentest
import (
        "context"
        "reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)
// A list of SSIS object metadata.
// API Version: 2018-06-01.
func GetIntegrationRuntimeObjectMetadatum(ctx *pulumi.Context, args *GetIntegrationRuntimeObjectMetadatumArgs, opts ...pulumi.InvokeOption) (*GetIntegrationRuntimeObjectMetadatumResult, error) {
	var rv GetIntegrationRuntimeObjectMetadatumResult
	err := ctx.Invoke("azure-native:datafactory:getIntegrationRuntimeObjectMetadatum", args, &rv, opts...)
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

type GetIntegrationRuntimeObjectMetadatumArgs struct {
	// The factory name.
	FactoryName string `pulumi:"factoryName"`
	// The integration runtime name.
	IntegrationRuntimeName string `pulumi:"integrationRuntimeName"`
	// Metadata path.
	MetadataPath *string `pulumi:"metadataPath"`
	// The resource group name.
	ResourceGroupName string `pulumi:"resourceGroupName"`
}


// A list of SSIS object metadata.
type GetIntegrationRuntimeObjectMetadatumResult struct {
	// The link to the next page of results, if any remaining results exist.
	NextLink *string `pulumi:"nextLink"`
	// List of SSIS object metadata.
	Value []interface{} `pulumi:"value"`
}


func GetIntegrationRuntimeObjectMetadatumApply(ctx *pulumi.Context, args GetIntegrationRuntimeObjectMetadatumApplyInput, opts ...pulumi.InvokeOption) GetIntegrationRuntimeObjectMetadatumResultOutput {
	return args.ToGetIntegrationRuntimeObjectMetadatumApplyOutput().ApplyT(func (v GetIntegrationRuntimeObjectMetadatumArgs) (GetIntegrationRuntimeObjectMetadatumResult, error) {
	r, err := GetIntegrationRuntimeObjectMetadatum(ctx, &v, opts...)
	return *r, err

}).(GetIntegrationRuntimeObjectMetadatumResultOutput)}

// GetIntegrationRuntimeObjectMetadatumApplyInput is an input type that accepts GetIntegrationRuntimeObjectMetadatumApplyArgs and GetIntegrationRuntimeObjectMetadatumApplyOutput values.
// You can construct a concrete instance of `GetIntegrationRuntimeObjectMetadatumApplyInput` via:
//
//          GetIntegrationRuntimeObjectMetadatumApplyArgs{...}
type GetIntegrationRuntimeObjectMetadatumApplyInput interface {
	pulumi.Input

	ToGetIntegrationRuntimeObjectMetadatumApplyOutput() GetIntegrationRuntimeObjectMetadatumApplyOutput
	ToGetIntegrationRuntimeObjectMetadatumApplyOutputWithContext(context.Context) GetIntegrationRuntimeObjectMetadatumApplyOutput
}

type GetIntegrationRuntimeObjectMetadatumApplyArgs struct {
	// The factory name.
	FactoryName pulumi.StringInput `pulumi:"factoryName"`
	// The integration runtime name.
	IntegrationRuntimeName pulumi.StringInput `pulumi:"integrationRuntimeName"`
	// Metadata path.
	MetadataPath pulumi.StringPtrInput `pulumi:"metadataPath"`
	// The resource group name.
	ResourceGroupName pulumi.StringInput `pulumi:"resourceGroupName"`
}

func (GetIntegrationRuntimeObjectMetadatumApplyArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*GetIntegrationRuntimeObjectMetadatumArgs)(nil)).Elem()
}

func (i GetIntegrationRuntimeObjectMetadatumApplyArgs) ToGetIntegrationRuntimeObjectMetadatumApplyOutput() GetIntegrationRuntimeObjectMetadatumApplyOutput {
	return i.ToGetIntegrationRuntimeObjectMetadatumApplyOutputWithContext(context.Background())
}

func (i GetIntegrationRuntimeObjectMetadatumApplyArgs) ToGetIntegrationRuntimeObjectMetadatumApplyOutputWithContext(ctx context.Context) GetIntegrationRuntimeObjectMetadatumApplyOutput {
	return pulumi.ToOutputWithContext(ctx, i).(GetIntegrationRuntimeObjectMetadatumApplyOutput)
}

type GetIntegrationRuntimeObjectMetadatumApplyOutput struct { *pulumi.OutputState }

func (GetIntegrationRuntimeObjectMetadatumApplyOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*GetIntegrationRuntimeObjectMetadatumArgs)(nil)).Elem()
}

func (o GetIntegrationRuntimeObjectMetadatumApplyOutput) ToGetIntegrationRuntimeObjectMetadatumApplyOutput() GetIntegrationRuntimeObjectMetadatumApplyOutput {
	return o
}

func (o GetIntegrationRuntimeObjectMetadatumApplyOutput) ToGetIntegrationRuntimeObjectMetadatumApplyOutputWithContext(ctx context.Context) GetIntegrationRuntimeObjectMetadatumApplyOutput {
	return o
}

// The factory name.
func (o GetIntegrationRuntimeObjectMetadatumApplyOutput) FactoryName() pulumi.StringOutput {
	return o.ApplyT(func (v GetIntegrationRuntimeObjectMetadatumArgs) string { return v.FactoryName }).(pulumi.StringOutput)
}

// The integration runtime name.
func (o GetIntegrationRuntimeObjectMetadatumApplyOutput) IntegrationRuntimeName() pulumi.StringOutput {
	return o.ApplyT(func (v GetIntegrationRuntimeObjectMetadatumArgs) string { return v.IntegrationRuntimeName }).(pulumi.StringOutput)
}

// Metadata path.
func (o GetIntegrationRuntimeObjectMetadatumApplyOutput) MetadataPath() pulumi.StringPtrOutput {
	return o.ApplyT(func (v GetIntegrationRuntimeObjectMetadatumArgs) *string { return v.MetadataPath }).(pulumi.StringPtrOutput)
}

// The resource group name.
func (o GetIntegrationRuntimeObjectMetadatumApplyOutput) ResourceGroupName() pulumi.StringOutput {
	return o.ApplyT(func (v GetIntegrationRuntimeObjectMetadatumArgs) string { return v.ResourceGroupName }).(pulumi.StringOutput)
}

// A list of SSIS object metadata.
type GetIntegrationRuntimeObjectMetadatumResultOutput struct { *pulumi.OutputState }

func (GetIntegrationRuntimeObjectMetadatumResultOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*GetIntegrationRuntimeObjectMetadatumResult)(nil)).Elem()
}

func (o GetIntegrationRuntimeObjectMetadatumResultOutput) ToGetIntegrationRuntimeObjectMetadatumResultOutput() GetIntegrationRuntimeObjectMetadatumResultOutput {
	return o
}

func (o GetIntegrationRuntimeObjectMetadatumResultOutput) ToGetIntegrationRuntimeObjectMetadatumResultOutputWithContext(ctx context.Context) GetIntegrationRuntimeObjectMetadatumResultOutput {
	return o
}

// The link to the next page of results, if any remaining results exist.
func (o GetIntegrationRuntimeObjectMetadatumResultOutput) NextLink() pulumi.StringPtrOutput {
	return o.ApplyT(func (v GetIntegrationRuntimeObjectMetadatumResult) *string { return v.NextLink }).(pulumi.StringPtrOutput)
}

// List of SSIS object metadata.
func (o GetIntegrationRuntimeObjectMetadatumResultOutput) Value() pulumi.ArrayOutput {
	return o.ApplyT(func (v GetIntegrationRuntimeObjectMetadatumResult) []interface{} { return v.Value }).(pulumi.ArrayOutput)
}

