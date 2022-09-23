// Copyright 2021, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		for i := 0; i < 10; i++ {
			fmt.Printf("Line %d\n", i)
			fmt.Fprintf(os.Stderr, "Errln %d\n", i+10)
		}
		fmt.Printf("Line 10")
		fmt.Fprintf(os.Stderr, "Errln 20")
		return nil
	})
}
