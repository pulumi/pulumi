// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		randomResourceName := fmt.Sprintf("random-%d", time.Now().UnixMilli()/1000)
		rand, err := NewRandom(ctx, randomResourceName, &RandomArgs{Length: pulumi.Int(10)})
		if err != nil {
			return err
		}

		// Echoed returns the input resource as an output
		echoed, err := NewEcho(ctx, "echo-random", &EchoArgs{Echo: pulumi.NewResourceInput(rand)})
		if err != nil {
			return err
		}

		output := echoed.Echo.ApplyT(func(v pulumi.Resource) pulumi.StringOutput {
			casted, ok := v.(*Random)
			if !ok {
				fmt.Printf("Failed to cast value to Random\n")
				return pulumi.Sprintf("empty")
			} else {
				return casted.Result
			}

		}).(pulumi.StringOutput)

		ctx.Export("randResult", rand.Result)
		ctx.Export("echoedResult", output)
		return nil
	})
}
