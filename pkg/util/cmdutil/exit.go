// Copyright 2017 Pulumi, Inc. All rights reserved.

package cmdutil

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/diag"
)

// RunFunc wraps an error-returning run func with standard Coconut error handling.  All Coconut commands should wrap
// themselves in this to ensure consistent and appropriate error behavior.  In particular, we want to avoid any calls to
// os.Exit in the middle of a callstack which might prohibit reaping of child processes, resources, etc.  And we wish to
// avoid the default Cobra unhandled error behavior, because it is formatted incorrectly and needlessly prints usage.
func RunFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := run(cmd, args); err != nil {
			ExitError(err.Error())
		}
	}
}

// ExitError issues an error and exits with a standard error exit code.
func ExitError(msg string, args ...interface{}) {
	ExitErrorCode(-1, msg, args...)
}

// ExitErrorCode issues an error and exists with the given error exit code.
func ExitErrorCode(code int, msg string, args ...interface{}) {
	Sink().Errorf(diag.Message(fmt.Sprintf(msg, args...)))
	os.Exit(code)
}
