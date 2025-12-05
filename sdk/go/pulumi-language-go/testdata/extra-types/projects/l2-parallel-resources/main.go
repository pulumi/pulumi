package main

import (
	"example.com/pulumi-sync/sdk/go/v3/sync"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := sync.NewBlock(ctx, "block-1", nil)
		if err != nil {
			return err
		}
		_, err = sync.NewBlock(ctx, "block-2", nil)
		if err != nil {
			return err
		}
		_, err = sync.NewBlock(ctx, "block-3", nil)
		if err != nil {
			return err
		}
		return nil
	})
}
