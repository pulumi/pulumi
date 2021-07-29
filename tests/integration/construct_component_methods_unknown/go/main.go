// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		r, err := NewRandom(ctx, "resource", &RandomArgs{Length: pulumi.Int(10)})
		if err != nil {
			return err
		}

		component, err := NewComponent(ctx, "component")
		if err != nil {
			return err
		}
		result, err := component.GetMessage(ctx, &ComponentGetMessageArgs{
			Echo: r.ID().ToStringOutput(),
		})
		if err != nil {
			return err
		}

		ctx.Export("result", result.ApplyT(func(val ComponentGetMessageResult) string {
			panic("should not run (result)")
		}))
		return nil
	})
}
