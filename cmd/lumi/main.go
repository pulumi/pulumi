// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-fabric/pkg/engine"
)

var (
	// The lumi engine provides an API for common lumi tasks.  It's shared across the
	// `lumi` command and the deployment engine in the pulumi-service. For `lumi` we set
	// the engine to write output and errors to os.Stdout and os.Stderr.
	lumiEngine engine.Engine
)

func init() {
	lumiEngine = engine.Engine{Stdout: os.Stdout, Stderr: os.Stderr, Environment: fileSystemEnvironmentProvider{}}
}

func main() {
	if err := NewLumiCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		os.Exit(-1)
	}
}
