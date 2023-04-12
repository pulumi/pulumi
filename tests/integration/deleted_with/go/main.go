// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		rand, err := NewRandom(ctx, "random", &RandomArgs{Length: pulumi.Int(10)})
		if err != nil {
			return err
		}

		_, err = NewFailsOnDelete(ctx, "failsondelete", pulumi.DeletedWith(rand))
		if err != nil {
			return err
		}

		return nil
	})
}
