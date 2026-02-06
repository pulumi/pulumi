// Copyright 2024, Pulumi Corporation.
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
	"context"
	"errors"
)

// ExitCoder is an interface for errors that carry a semantic exit code.
type ExitCoder interface {
	error
	ExitCode() int
}

// CLIError wraps an error with a semantic exit code.
type CLIError struct {
	code int
	err  error
}

func (e *CLIError) Error() string { return e.err.Error() }
func (e *CLIError) ExitCode() int { return e.code }
func (e *CLIError) Unwrap() error { return e.err }

// WrapWithExitCode wraps an error with a semantic exit code. If err is nil,
// nil is returned.
func WrapWithExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &CLIError{code: code, err: err}
}

// ExitCodeFor extracts the semantic exit code from an error chain.
// It returns ExitSuccess (0) for nil errors, checks for the ExitCoder
// interface, then context.Canceled, and defaults to ExitCodeError (1).
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var exitCoder ExitCoder
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}

	if errors.Is(err, context.Canceled) {
		return ExitCancelled
	}

	return ExitCodeError
}

// CancellationError represents a user-initiated cancellation of an operation.
type CancellationError struct {
	Operation string
}

func (e CancellationError) Error() string {
	return e.Operation + " cancelled"
}

func (e CancellationError) ExitCode() int {
	return ExitCancelled
}
