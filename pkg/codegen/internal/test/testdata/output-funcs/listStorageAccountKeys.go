package codegentest
import (
        "context"
        "reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)
// The response from the ListKeys operation.
func ListStorageAccountKeys(ctx *pulumi.Context, args *ListStorageAccountKeysArgs, opts ...pulumi.InvokeOption) (*ListStorageAccountKeysResult, error) {
	var rv ListStorageAccountKeysResult
	err := ctx.Invoke("azure-native:storage/v20210101:listStorageAccountKeys", args, &rv, opts...)
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

type ListStorageAccountKeysArgs struct {
	// The name of the storage account within the specified resource group. Storage account names must be between 3 and 24 characters in length and use numbers and lower-case letters only.
	AccountName string `pulumi:"accountName"`
	// Specifies type of the key to be listed. Possible value is kerb.
	Expand *string `pulumi:"expand"`
	// The name of the resource group within the user's subscription. The name is case insensitive.
	ResourceGroupName string `pulumi:"resourceGroupName"`
}


// The response from the ListKeys operation.
type ListStorageAccountKeysResult struct {
	// Gets the list of storage account keys and their properties for the specified storage account.
	Keys []map[string]string `pulumi:"keys"`
}


func ListStorageAccountKeysApply(ctx *pulumi.Context, args ListStorageAccountKeysApplyInput, opts ...pulumi.InvokeOption) ListStorageAccountKeysResultOutput {
	return args.ToListStorageAccountKeysApplyOutput().ApplyT(func (v ListStorageAccountKeysArgs) (ListStorageAccountKeysResult, error) {
	r, err := ListStorageAccountKeys(ctx, &v, opts...)
	return *r, err

}).(ListStorageAccountKeysResultOutput)}

// ListStorageAccountKeysApplyInput is an input type that accepts ListStorageAccountKeysApplyArgs and ListStorageAccountKeysApplyOutput values.
// You can construct a concrete instance of `ListStorageAccountKeysApplyInput` via:
//
//          ListStorageAccountKeysApplyArgs{...}
type ListStorageAccountKeysApplyInput interface {
	pulumi.Input

	ToListStorageAccountKeysApplyOutput() ListStorageAccountKeysApplyOutput
	ToListStorageAccountKeysApplyOutputWithContext(context.Context) ListStorageAccountKeysApplyOutput
}

type ListStorageAccountKeysApplyArgs struct {
	// The name of the storage account within the specified resource group. Storage account names must be between 3 and 24 characters in length and use numbers and lower-case letters only.
	AccountName pulumi.StringInput `pulumi:"accountName"`
	// Specifies type of the key to be listed. Possible value is kerb.
	Expand pulumi.StringPtrInput `pulumi:"expand"`
	// The name of the resource group within the user's subscription. The name is case insensitive.
	ResourceGroupName pulumi.StringInput `pulumi:"resourceGroupName"`
}

func (ListStorageAccountKeysApplyArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*ListStorageAccountKeysArgs)(nil)).Elem()
}

func (i ListStorageAccountKeysApplyArgs) ToListStorageAccountKeysApplyOutput() ListStorageAccountKeysApplyOutput {
	return i.ToListStorageAccountKeysApplyOutputWithContext(context.Background())
}

func (i ListStorageAccountKeysApplyArgs) ToListStorageAccountKeysApplyOutputWithContext(ctx context.Context) ListStorageAccountKeysApplyOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ListStorageAccountKeysApplyOutput)
}

type ListStorageAccountKeysApplyOutput struct { *pulumi.OutputState }

func (ListStorageAccountKeysApplyOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ListStorageAccountKeysArgs)(nil)).Elem()
}

func (o ListStorageAccountKeysApplyOutput) ToListStorageAccountKeysApplyOutput() ListStorageAccountKeysApplyOutput {
	return o
}

func (o ListStorageAccountKeysApplyOutput) ToListStorageAccountKeysApplyOutputWithContext(ctx context.Context) ListStorageAccountKeysApplyOutput {
	return o
}

// The name of the storage account within the specified resource group. Storage account names must be between 3 and 24 characters in length and use numbers and lower-case letters only.
func (o ListStorageAccountKeysApplyOutput) AccountName() pulumi.StringOutput {
	return o.ApplyT(func (v ListStorageAccountKeysArgs) string { return v.AccountName }).(pulumi.StringOutput)
}

// Specifies type of the key to be listed. Possible value is kerb.
func (o ListStorageAccountKeysApplyOutput) Expand() pulumi.StringPtrOutput {
	return o.ApplyT(func (v ListStorageAccountKeysArgs) *string { return v.Expand }).(pulumi.StringPtrOutput)
}

// The name of the resource group within the user's subscription. The name is case insensitive.
func (o ListStorageAccountKeysApplyOutput) ResourceGroupName() pulumi.StringOutput {
	return o.ApplyT(func (v ListStorageAccountKeysArgs) string { return v.ResourceGroupName }).(pulumi.StringOutput)
}

// The response from the ListKeys operation.
type ListStorageAccountKeysResultOutput struct { *pulumi.OutputState }

func (ListStorageAccountKeysResultOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ListStorageAccountKeysResult)(nil)).Elem()
}

func (o ListStorageAccountKeysResultOutput) ToListStorageAccountKeysResultOutput() ListStorageAccountKeysResultOutput {
	return o
}

func (o ListStorageAccountKeysResultOutput) ToListStorageAccountKeysResultOutputWithContext(ctx context.Context) ListStorageAccountKeysResultOutput {
	return o
}

// Gets the list of storage account keys and their properties for the specified storage account.
func (o ListStorageAccountKeysResultOutput) Keys() pulumi.StringMapArrayOutput {
	return o.ApplyT(func (v ListStorageAccountKeysResult) []map[string]string { return v.Keys }).(pulumi.StringMapArrayOutput)
}

