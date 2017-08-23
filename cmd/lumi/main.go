// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-fabric/pkg/engine"
)

func main() {
	engine.InitStreams(os.Stdout, os.Stderr)

	if err := NewLumiCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		os.Exit(-1)
	}
}
