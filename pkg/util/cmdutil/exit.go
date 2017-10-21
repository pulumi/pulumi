// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/diag"
)

// RunFunc wraps an error-returning run func with standard Pulumi error handling.  All Lumi commands should wrap
// themselves in this to ensure consistent and appropriate error behavior.  In particular, we want to avoid any calls to
// os.Exit in the middle of a callstack which might prohibit reaping of child processes, resources, etc.  And we wish to
// avoid the default Cobra unhandled error behavior, because it is formatted incorrectly and needlessly prints usage.
func RunFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := run(cmd, args); err != nil {
			msg := err.Error()
			if stackerr, ok := err.(interface {
				StackTrace() errors.StackTrace
			}); ok {
				// If there is a stack trace, and logging is enabled, append it.  Otherwise, debug glog it.
				stack := msg + "\n"
				for _, f := range stackerr.StackTrace() {
					stack += fmt.Sprintf("%+s\n", f)
				}
				if LogToStderr {
					msg = stack
				} else {
					glog.V(3).Infof(stack)
				}
			}
			ExitError(msg)
		}
	}
}

// ExitError issues an error and exits with a standard error exit code.
func ExitError(msg string, args ...interface{}) {
	ExitErrorCode(-1, msg, args...)
}

// ExitErrorCode issues an error and exists with the given error exit code.
func ExitErrorCode(code int, msg string, args ...interface{}) {
	Diag().Errorf(diag.Message(fmt.Sprintf(msg, args...)))
	os.Exit(code)
}
