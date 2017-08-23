// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-fabric/pkg/engine"
)

var (
	lumiEngine engine.Engine
)

func init() {
	lumiEngine = engine.Engine{Stdout: os.Stdout, Stderr: os.Stderr}
}

func main() {
	if err := NewLumiCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		os.Exit(-1)
	}
}
