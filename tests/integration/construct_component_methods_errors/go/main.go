// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		component, err := NewComponent(ctx, "component")
		if err != nil {
			return err
		}
		_, err = component.GetMessage(ctx, &ComponentGetMessageArgs{
			Echo: pulumi.String("hello"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
