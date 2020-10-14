package myrandom

import (
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"

	"github.com/pulumi/pulumi/pkg/v2/codegen/go/provider/testdata/random/sdk/go/myrandom"
)

type HashComponentArgs struct {
	// Count indicates the number of random bytes to hash.
	Count pulumi.IntInput `pulumi:"count"`
}

// HashComponent is a component resource that generates a CRC-32 hash from a given number of random bytes.
type HashComponent struct {
	pulumi.ResourceState

	// The hash.
	Hash pulumi.IntOutput `pulumi:"hash"`
}

//pulumi:constructor
func NewHashComponent(ctx *pulumi.Context, name string, args *HashComponentArgs, opts ...pulumi.ResourceOption) (*HashComponent, error) {
	var resource HashComponent
	if err := ctx.RegisterComponentResource(&resource, "myrandom::hash", name, opts...); err != nil {
		return nil, err
	}

	bytes, err := myrandom.NewRandomBytes(ctx, name+"-bytes", &myrandom.RandomBytesArgs{
		Count: args,
	})
	if err != nil {
		return nil, err
	}

	resource.Hash = bytes.Bytes.ApplyT(func(bytes []byte) int {
		return int(crc32.ChecksumIEEE(bytes))
	}).(pulumi.IntOutput)

	ctx.RegisterResourceOutputs(&resource, pulumi.Map{
		"hash": resource.Hash,
	})

	return resource, nil
}
