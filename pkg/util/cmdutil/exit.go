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

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/logging"
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

// runPostCommandHooks runs any post-hooks present on the given cobra.Command. This logic is copied directly from
// cobra itself; see https://github.com/spf13/cobra/blob/4dab30cb33e6633c33c787106bafbfbfdde7842d/command.go#L768-L785
// for the original.
func runPostCommandHooks(c *cobra.Command, args []string) error {
	if c.PostRunE != nil {
		if err := c.PostRunE(c, args); err != nil {
			return err
		}
	} else if c.PostRun != nil {
		c.PostRun(c, args)
	}
	for p := c; p != nil; p = p.Parent() {
		if p.PersistentPostRunE != nil {
			if err := p.PersistentPostRunE(c, args); err != nil {
				return err
			}
			break
		} else if p.PersistentPostRun != nil {
			p.PersistentPostRun(c, args)
			break
		}
	}
	return nil
}

// RunFunc wraps an error-returning run func with standard Pulumi error handling.  All Pulumi commands should wrap
// themselves in this to ensure consistent and appropriate error behavior.  In particular, we want to avoid any calls to
// os.Exit in the middle of a callstack which might prohibit reaping of child processes, resources, etc.  And we wish to
// avoid the default Cobra unhandled error behavior, because it is formatted incorrectly and needlessly prints usage.
func RunFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := run(cmd, args); err != nil {
			// Sadly, the fact that we hard-exit below means that it's up to us to replicate the Cobra post-run
			// behavior here.
			if postRunErr := runPostCommandHooks(cmd, args); postRunErr != nil {
				err = multierror.Append(err, postRunErr)
			}

			// If there is a stack trace, and logging is enabled, append it.  Otherwise, debug logging it.
			var msg string
			if logging.LogToStderr {
				msg = DetailedError(err)
			} else {
				msg = errorMessage(err)
				logging.V(3).Infof(DetailedError(err))
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
	Diag().Errorf(diag.Message("", msg), false, args...)
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
			msg += fmt.Sprintf("\n    %d) %s", i+1, errorMessage(werr))
		}
		return msg
	}
	return err.Error()
}
