// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/diag"
)

// DetailedError extracts a detailed error message, including stack trace, if there is one.
func DetailedError(err error) string {
	msg := errorMessage(err)
	hasstack := false
	for {
		if stackerr, ok := err.(interface {
			StackTrace() errors.StackTrace
		}); ok {
			msg += "\n"
			if hasstack {
				msg += "CAUSED BY...\n"
			}
			hasstack = true

			// Append the stack trace.
			for _, f := range stackerr.StackTrace() {
				msg += fmt.Sprintf("%+v\n", f)
			}

			// Keep going up the causer chain, if any.
			cause := errors.Cause(err)
			if cause == err || cause == nil {
				break
			}
			err = cause
		} else {
			break
		}
	}
	return msg
}

// RunFunc wraps an error-returning run func with standard Pulumi error handling.  All Pulumi commands should wrap
// themselves in this to ensure consistent and appropriate error behavior.  In particular, we want to avoid any calls to
// os.Exit in the middle of a callstack which might prohibit reaping of child processes, resources, etc.  And we wish to
// avoid the default Cobra unhandled error behavior, because it is formatted incorrectly and needlessly prints usage.
func RunFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := run(cmd, args); err != nil {
			// If there is a stack trace, and logging is enabled, append it.  Otherwise, debug glog it.
			var msg string
			if LogToStderr {
				msg = DetailedError(err)
			} else {
				msg = errorMessage(err)
				glog.V(3).Infof(DetailedError(err))
			}
			ExitError(msg)
		}
	}
}

// Exit exits with a given error.
func Exit(err error) {
	ExitError(errorMessage(err))
}

// ExitError issues an error and exits with a standard error exit code.
func ExitError(msg string, args ...interface{}) {
	exitErrorCode(-1, msg, args...)
}

// exitErrorCode issues an error and exists with the given error exit code.
func exitErrorCode(code int, msg string, args ...interface{}) {
	Diag().Errorf(diag.Message("", msg), args...)
	os.Exit(code)
}

// errorMessage returns a message, possibly cleaning up the text if appropriate.
func errorMessage(err error) string {
	if multi, ok := err.(*multierror.Error); ok {
		wr := multi.WrappedErrors()
		if len(wr) == 1 {
			return errorMessage(wr[0])
		}
		msg := fmt.Sprintf("%d errors occurred:", len(wr))
		for i, werr := range wr {
			msg += fmt.Sprintf("\n    %d) %s", i, errorMessage(werr))
		}
		return msg
	}
	return err.Error()
}
