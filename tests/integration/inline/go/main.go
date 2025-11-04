//go:build !all
// +build !all

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/inline"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		r, err := NewRandom(ctx, "foo")
		ctx.Export("random_id", r.ID())
		ctx.Export("random_result", r.Result)
		return err
	})
}

type Random struct {
	pulumi.CustomResourceState
	Result pulumi.StringOutput `pulumi:"result"`
}

func NewRandom(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Random, error) {
	var random Random
	err := ctx.RegisterInlineResource(inline.Provider{
		Create: func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
			bytes := make([]byte, 15)
			if _, err := rand.Read(bytes); err != nil {
				return plugin.CreateResponse{}, err
			}
			result := hex.EncodeToString(bytes)

			return plugin.CreateResponse{
				ID: resource.ID(result),
				Properties: resource.PropertyMap{
					"result": resource.NewProperty(result),
				},
			}, nil
		},
	}, name, nil, &random, opts...)
	return &random, err
}
