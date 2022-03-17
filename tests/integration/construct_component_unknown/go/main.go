// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		r, err := NewRandom(ctx, "resource", &RandomArgs{Length: pulumi.Int(10)})
		if err != nil {
			return err
		}
		_, err = NewComponent(ctx, "component", &ComponentArgs{
			Message: r.ID().ApplyT(func(id pulumi.ID) string {
				return fmt.Sprintf("message %v", id)
			}).(pulumi.StringOutput),
			Nested: &ComponentNestedArgs{
				Value: r.ID().ApplyT(func(id pulumi.ID) string {
					return fmt.Sprintf("nested.value %v", id)
				}).(pulumi.StringOutput),
			},
		})
		return err
	})
}
