package main

import (
	"github.com/pulumi/pulumi-unknown/sdk/go/unknown"
	"github.com/pulumi/pulumi-unknown/sdk/go/unknown/eks"
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
		_, err = eks.ModuleValues(ctx, map[string]interface{}{}, nil)
		if err != nil {
			return err
		}
		ctx.Export("content", data.Content)
		return nil
	})
}
