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

package result

import "github.com/pkg/errors"

// Result represents the result of a computation that can fail. The Result type
// revolves around two notions of failure:
//
// 1. Computations can fail, but they can fail gracefully. Computations that
// fail gracefully do so by logging a diagnostic and returning a non-nil "bail"
// result.
//
// 2. Computations can fail due to bugs in Pulumi. Computations that fail in
// this manner do so by constructing a Result using the `Error`, `Errorf`, or
// `FromError` constructor functions.
//
// Result is best used as a pointer type so that it can be nullable. A function
// returning a pointer Result has the following semantics:
//
//  * If the result is `nil`, the caller should proceed. The callee believes
//  that the overarching plan can still continue, even if it logged
//  diagnostics.
//
//  * If the result is non-nil, the caller should not proceed.  Most often, the
//  caller should return this Result to its caller.
//
// At the highest level, when a function wishes to return only an `error`, the
// `Error` member function can be used to turn a nullable `Result` into an
// `error`.
type Result struct {
	bail bool
	err  error
}

// Error produces an `error` from this Result. Returns nil unless the provided
// Result represents an internal-to-Pulumi error.
func (r *Result) Error() error {
	if r != nil && !r.bail {
		return r.err
	}

	return nil
}

// Bail produces a Result that represents a computation that failed to complete
// successfully but is not a bug in Pulumi.
func Bail() *Result {
	return &Result{bail: true}
}

// Errorf produces a Result that represents an internal Pulumi error,
// constructed from the given format string and arguments.
func Errorf(msg string, args ...interface{}) *Result {
	err := errors.Errorf(msg, args...)
	return FromError(err)
}

// Error produces a Result that represents an internal Pulumi error,
// constructed from the given message.
func Error(msg string) *Result {
	err := errors.New(msg)
	return FromError(err)
}

// FromError produces a Result that wraps an internal Pulumi error.
func FromError(err error) *Result {
	return &Result{err: err}
}

// TODO returns an error that can be used in places that have not yet been
// adapted to use Results.  Their use is intended to be temporary until Results
// are plumbed throughout the Pulumi codebase.
func TODO() error {
	return errors.New("bailng due to error")
}
