// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/diag"
)

func main() {
	if err := NewCocoCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		os.Exit(-1)
	}
}

var snk diag.Sink

// sink lazily allocates a sink to be used if we can't create a compiler.
func sink() diag.Sink {
	if snk == nil {
		snk = core.DefaultSink("")
	}
	return snk
}

// runFunc wraps an error-returning run func with standard Coconut error handling.  All Coconut commands should wrap
// themselves in this to ensure consistent and appropriate error behavior.  In particular, we want to avoid any calls to
// os.Exit in the middle of a callstack which might prohibit reaping of child processes, resources, etc.  And we wish to
// avoid the default Cobra unhandled error behavior, because it is formatted incorrectly and needlessly prints usage.
func runFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := run(cmd, args); err != nil {
			exitError(err.Error())
		}
	}
}

// exitError issues an error and exits with a standard error exit code.
func exitError(msg string, args ...interface{}) {
	exitErrorCode(-1, msg, args...)
}

// exitErrorCode issues an error and exists with the given error exit code.
func exitErrorCode(code int, msg string, args ...interface{}) {
	sink().Errorf(diag.Message(fmt.Sprintf(msg, args...)))
	os.Exit(code)
}
