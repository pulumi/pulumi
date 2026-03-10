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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// DetailedError extracts a detailed error message, including stack trace, if there is one.
func DetailedError(err error) string {
	var msg strings.Builder
	msg.WriteString(errorMessage(err))

	hasstack := false

	for {
		if stackerr, ok := err.(interface {
			StackTrace() pkgerrors.StackTrace
		}); ok {
			msg.WriteString("\n")
			if hasstack {
				msg.WriteString("CAUSED BY...\n")
			}
			hasstack = true

			// Append the stack trace.
			for _, f := range stackerr.StackTrace() {
				msg.WriteString(fmt.Sprintf("%+v\n", f))
			}

			// Keep going up the causer chain, if any.
			cause := pkgerrors.Cause(err)
			if cause == err || cause == nil {
				break
			}
			err = cause
		} else {
			break
		}
	}
	return msg.String()
}

// RunFunc wraps an error-returning run func with standard Pulumi error handling.  All
// Pulumi commands should wrap themselves in this to ensure consistent and appropriate
// error behavior.  In particular, we want to avoid any calls to os.Exit in the middle of
// a callstack which might prohibit reaping of child processes, resources, etc.  And we
// wish to avoid the default Cobra unhandled error behavior, because it is formatted
// incorrectly and needlessly prints usage.
//
// If run returns a BailError, we will not print an error message.
//
// Deprecated: Instead of using [RunFunc], you should call [DisplayErrorMessage] and then
// manually exit with `os.Exit(-1)`
func RunFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		err := run(cmd, args)
		if err != nil {
			DisplayErrorMessage(err)
			os.Exit(-1)
		}
	}
}

// DisplayErrorMessage displays an error message to the user.
//
// DisplayErrorMessage respects [result.IsBail] and [logging.LogToStderr].
func DisplayErrorMessage(err error) {
	// If we were asked to bail, that means we already printed out a message.  We just need
	// to quit at this point (with an error code so no one thinks we succeeded).  Bailing
	// always indicates a failure, just one we don't need to print a message for.
	if err == nil || result.IsBail(err) {
		return
	}

	var msg string
	if logging.LogToStderr {
		msg = DetailedError(err)
	} else {
		msg = errorMessage(err)
		logging.V(3).Info(DetailedError(err))
	}

	Diag().Errorf(diag.Message("", "%s"), msg)
}

// Exit exits with a given error.
func Exit(err error) {
	ExitError(errorMessage(err))
}

// ExitError issues an error and exits with a standard error exit code.
func ExitError(msg string) {
	Diag().Errorf(diag.Message("", "%s"), msg)
	os.Exit(ExitCodeError)
}

// Exit code taxonomy for the Pulumi CLI. These values form part of the
// contract for automation and agent integrations and must be treated as
// stable once released.
const (
	ExitSuccess            = 0
	ExitCodeError          = 1 // generic/unclassified error
	ExitConfigurationError = 2 // invalid flags, config, or invocation
	ExitAuthenticationError = 3
	ExitResourceError       = 4
	ExitPolicyViolation     = 5
	ExitStackNotFound       = 6
	ExitNoChanges           = 7
	ExitCancelled           = 8
	ExitTimeout             = 9
	ExitInternalError       = 255
)

// ExitCodeFor maps an error to a process exit code, based on its concrete or
// wrapped type. This function is the single choke point for mapping errors
// into the public exit code contract.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// Respect bail semantics first – still a failure, but don't print anything.
	if result.IsBail(err) {
		return ExitCodeError
	}

	// Authentication / authorization problems.
	switch {
	case errors.As(err, &backenderr.LoginRequiredError{}),
		errors.As(err, &backenderr.ForbiddenError{}),
		errors.As(err, &backenderr.MissingEnvVarForNonInteractiveError{}):
		return ExitAuthenticationError
	}

	// Stack context problems.
	switch {
	case errors.As(err, &backenderr.StackNotFoundError{}),
		errors.As(err, &backenderr.NoStacksError{}),
		errors.As(err, &backenderr.NoStackSelectedError{}),
		errors.As(err, &backenderr.StackStateNotFoundError{}):
		return ExitStackNotFound
	}

	// User-initiated cancellation.
	if errors.As(err, &backenderr.CancelledError{}) {
		return ExitCancelled
	}

	// Expectation / invariant failures like --expect-no-changes.
	if errors.As(err, &backenderr.NoChangesExpectedError{}) {
		return ExitNoChanges
	}

	// Non-interactive mode without confirmation flags.
	if errors.As(err, &backenderr.NoConfirmationInNonInteractiveError{}) {
		return ExitConfigurationError
	}

	// TODO: Wire policy, timeout, and resource-specific errors as they gain
	// concrete types in engine/backend layers.

	// Internal/unexpected errors can be identified here in future; for now,
	// fall back to the generic error exit code.
	return ExitCodeError
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
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("%d errors occurred:", len(underlying)))
		for i, werr := range underlying {
			msg.WriteString(fmt.Sprintf("\n    %d) %s", i+1, errorMessage(werr)))
		}
		return msg.String()
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
