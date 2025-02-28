// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdutil

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
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

// RunFunc is like [RunFuncE], but it calls [os.Exit] when an error is encountered.
//
// Deprecated: Please switch to RunFuncE instead:
//
//	-Run: cmd.RunFunc(func(cmd *cobra.Command, args []string) error {
//	+RunE: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
//
// RunFuncE allows better testing because it doesn't call [os.Exit] on an error.
func RunFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		f := RunFuncE(run)
		if f(cmd, args) != nil {
			// RunFuncE has already displayed any error message that needs to
			// be displayed, so we just exit.
			os.Exit(-1)
		}
	}
}

// RunFuncE wraps an error-returning run func with standard Pulumi error handling.  All
// Pulumi commands should wrap themselves in this to ensure consistent and appropriate
// error behavior.  In particular, we want to avoid any calls to os.Exit in the middle of
// a callstack which might prohibit reaping of child processes, resources, etc.  And we
// wish to avoid the default Cobra unhandled error behavior, because it is formatted
// incorrectly and needlessly prints usage.
//
// If run returns a BailError, we will not print an error message.
func RunFuncE(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true

		err := run(cmd, args)
		if err == nil {
			return nil
		}

		// If we were asked to bail, that means we already printed out a message.  We just need
		// to quit at this point (with an error code so no one thinks we succeeded).  Bailing
		// always indicates a failure, just one we don't need to print a message for.
		if result.IsBail(err) {
			// We return the error so a non-zero exit code is printed.
			return err
		}

		var msg string
		if logging.LogToStderr {
			msg = DetailedError(err)
		} else {
			msg = errorMessage(err)
			logging.V(3).Info(DetailedError(err))
		}

		Diag().Errorf(diag.Message("", "%s"), msg)
		return err
	}
}

// Exit exits with a given error.
func Exit(err error) {
	ExitError(errorMessage(err))
}

// ExitError issues an error and exits with a standard error exit code.
func ExitError(msg string) {
	Diag().Errorf(diag.Message("", "%s"), msg)
	os.Exit(-1)
}

// errorMessage returns a message, possibly cleaning up the text if appropriate.
func errorMessage(err error) string {
	contract.Requiref(err != nil, "err", "must not be nil")

	underlying := flattenErrors(err)

	switch len(underlying) {
	case 0:
		return err.Error()

	case 1:
		return underlying[0].Error()

	default:
		msg := fmt.Sprintf("%d errors occurred:", len(underlying))
		for i, werr := range underlying {
			msg += fmt.Sprintf("\n    %d) %s", i+1, errorMessage(werr))
		}
		return msg
	}
}

// Flattens an error into a slice of errors containing the supplied error and
// all errors it wraps. If the set of wrapped errors is a tree (as e.g. produced
// by errors.Join), the errors are flattened in a depth-first manner. This
// function supports both native wrapped errors and those produced by the
// multierror package.
func flattenErrors(err error) []error {
	var errs []error
	switch multi := err.(type) {
	case *multierror.Error:
		for _, e := range multi.Errors {
			errs = append(errs, flattenErrors(e)...)
		}
	case interface{ Unwrap() []error }:
		for _, e := range multi.Unwrap() {
			errs = append(errs, flattenErrors(e)...)
		}
	default:
		errs = append(errs, err)
	}
	return errs
}
