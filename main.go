// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/coconut/cmd"
)

func main() {
	if err := cmd.NewCoconutCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		os.Exit(-1)
	}
}
