package main

import (
	"github.com/pulumi/pulumi-unknown/sdk/v1/go/unknown"
	"github.com/pulumi/pulumi-unknown/sdk/v1/go/unknown/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		data, err := unknown.GetData(ctx, map[string]interface{}{
			"input": "hello",
		}, nil)
		if err != nil {
			return err
		}
		_, err = eks.ModuleValues(ctx, nil, nil)
		if err != nil {
			return err
		}
		ctx.Export("content", data.Content)
		return nil
	})
}
