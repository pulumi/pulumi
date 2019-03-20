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

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

// Result represents the result of a computation that can fail. The Result type revolves around two
// notions of failure:
//
// 1. Computations can fail, but they can fail gracefully. Computations that fail gracefully do so
// by logging a diagnostic and returning a non-nil "bail" result.
//
// 2. Computations can fail due to bugs in Pulumi. Computations that fail in this manner do so by
// constructing a Result using the `Error`, `Errorf`, or `FromError` constructor functions.
//
// Result is an interface so that it can be nullable. A function returning a pointer Result has the
// following semantics:
//
//  * If the result is `nil`, the caller should proceed. The callee believes
//  that the overarching plan can still continue, even if it logged
//  diagnostics.
//
//  * If the result is non-nil, the caller should not proceed.  Most often, the
//  caller should return this Result to its caller.
//
// At the highest level, when a function wishes to return only an `error`, the `Error` member
// function can be used to turn a nullable `Result` into an `error`.
type Result interface {
	Error() error
	IsBail() bool
}

type simpleResult struct {
	err error
}

func (r *simpleResult) Error() error { return r.err }
func (r *simpleResult) IsBail() bool { return r.err == nil }

// Bail produces a Result that represents a computation that failed to complete
// successfully but is not a bug in Pulumi.
func Bail() Result {
	return &simpleResult{err: nil}
}

// Errorf produces a Result that represents an internal Pulumi error,
// constructed from the given format string and arguments.
func Errorf(msg string, args ...interface{}) Result {
	err := errors.Errorf(msg, args...)
	return FromError(err)
}

// Error produces a Result that represents an internal Pulumi error,
// constructed from the given message.
func Error(msg string) Result {
	err := errors.New(msg)
	return FromError(err)
}

// FromError produces a Result that wraps an internal Pulumi error.  Do not call this with a 'nil'
// error.  A 'nil' error means that there was no problem, and in that case a 'nil' result should be
// used instead.
func FromError(err error) Result {
	if err == nil {
		panic("FromError should not be called with a nil-error.  " +
			"If there is no error, then a nil result should be returned.  " +
			"Caller should check for this first.")
	}

	return &simpleResult{err: err}
}

// WrapIfNonNil returns a non-nil Result if [err] is non-nil.  Otherwise it returns nil.
func WrapIfNonNil(err error) Result {
	if err == nil {
		return nil
	}

	return FromError(err)
}

// TODO returns an error that can be used in places that have not yet been
// adapted to use Results.  Their use is intended to be temporary until Results
// are plumbed throughout the Pulumi codebase.
func TODO() error {
	return errors.New("bailing due to error")
}

// Merge combines two results into one final result.  It properly respects all three forms of Result
// (i.e. nil/bail/error) for both results, and combines all sensibly into a final form that represents
// the information of both.
func Merge(res1 Result, res2 Result) Result {
	switch {
	// If both are nil, then there's no problem.  Return 'nil' to properly convey that outwards.
	case res1 == nil && res2 == nil:
		return nil

	// Otherwise, if one is nil, and the other is not, then the non-nil takes precedence.
	// i.e. an actual error (or bail) takes precedence
	case res1 == nil:
		return res2
	case res2 == nil:
		return res1

	// If both results have asked to bail, then just bail.  That properly respects both requests.
	case res1.IsBail() && res2.IsBail():
		return Bail()

	// We have two non-nil results and one, or both, of the results indicate an error.

	// If we have a request to Bail and a request to error then the request to error takes
	// precedence. The concept of bailing is that we've printed an error already and should just
	// quickly finish the entire pulumi execution.  However, for an error, we are indicating a bug
	// happened, and that we haven't printed it, and that it should print at the end.  So we need
	// to respect the error form here and pass it all the way back.
	case res1.IsBail():
		return res2
	case res2.IsBail():
		return res1

	// Both results are errors.  Combine them into one joint error and return that.
	default:
		return FromError(multierror.Append(res1.Error(), res2.Error()))
	}
}
