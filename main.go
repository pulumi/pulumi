// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func main() {
	if err := cmd.NewPulumiCmd().Execute(); err != nil {
		_, err = fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		contract.IgnoreError(err)
		os.Exit(-1)
	}
}
