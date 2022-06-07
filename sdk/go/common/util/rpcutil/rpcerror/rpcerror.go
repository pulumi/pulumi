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

// Package rpcerror provides helper types and functions for dealing with errors
// that cross gRPC boundaries.
//
// gRPC best practices dictate that the only error that should ever be returned
// by RPC server endpoints is `status.Status`. If an RPC server does not do this,
// gRPC will wrap it in a `status.Status` with an error code of Unknown, which is
// not useful to clients. This package provides a few functions, namely
// `New`, `Newf`, `Wrap`, and `Wrapf`, which provide RPC servers an easy way to wrap
// up existing errors or create new errors to return from RPC endpoints.
//
// For the client side, this package provides functions `FromError` and `Convert`,
// as well as types `Error` and `ErrorCause`, which allows RPC clients to inspect
// the error that occurred (including its error status) and, if one was provided,
// the cause of the error (i.e. the one that was wrapped via `Wrap` or `Wrapf`).
package rpcerror

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Error represents an error response from an RPC server endpoint.
// It contains a gRPC error code, a message, and a chain of "wrapped"
// errors that led to the final dispatch of this particular error message.
type Error struct {
	code    codes.Code
	message string
	cause   *ErrorCause
	details []interface{}
}

var _ error = (*Error)(nil)

func (r *Error) Error() string {
	if r.cause != nil {
		return fmt.Sprintf("%s: %s", r.message, r.cause.Message())
	}

	return r.message
}

// Code returns the gRPC error code for this error.
func (r *Error) Code() codes.Code {
	return r.code
}

// Message returns the message associated with this error cause.
func (r *Error) Message() string {
	return r.message
}

// Cause returns the error that was the root cause of this error,
// or nil if one wasn't provided.
func (r *Error) Cause() *ErrorCause {
	return r.cause
}

// Details returns the list of all auxiliary protobuf objects that were
// attached to this error by the server. It's up to the caller to try and
// downcast them to look for the one they are interested in.
func (r *Error) Details() []interface{} {
	return r.details
}

// ErrorCause represents a root cause of an error that ultimately caused
// an RPC endpoint to issue an error. ErrorCauses are optionally attached
// to Errors.
//
// All ErrorCauses have messages, but only a subset of them have stack traces.
// Notably, the pkg/errors package will affix stack traces to errors created through
// the errors.New and errors.Wrap.
type ErrorCause struct {
	message    string
	stackTrace string
}

// Message returns the message associated with this error cause.
func (r *ErrorCause) Message() string {
	return r.message
}

// StackTrace returns the stack trace associated with this error, or
// the empty string if one wasn't provided.
func (r *ErrorCause) StackTrace() string {
	return r.stackTrace
}

// New creates a new gRPC-compatible `error` with the given error code
// and message.
func New(code codes.Code, message string) error {
	status := status.New(code, message)
	return status.Err()
}

// Newf creates a new gRPC-compatible `error` with the given code and
// formatted message.
func Newf(code codes.Code, messageFormat string, args ...interface{}) error {
	status := status.Newf(code, messageFormat, args...)
	return status.Err()
}

// Wrap wraps an `error` into a gRPC-compatible `error`, recording the
// warpped error as the "cause" of the returned error.
//
// It is a logic error to call this function on an error previously
// returned by `rpcerrors.Wrap`.
func Wrap(code codes.Code, err error, message string) error {
	status := status.New(code, message)
	cause := serializeErrorCause(err)
	status, newErr := status.WithDetails(cause)
	contract.AssertNoError(newErr)
	return status.Err()
}

// Wrapf wraps an `error` into a gRPC-compatible `error`, plus a formatted message,
// recording the wrapped error as the "cause" of the returned error.
//
// It is a logic error to call this function on an error previously
// returned by `rpcerrors.Wrap`.
func Wrapf(code codes.Code, err error, messageFormat string, args ...interface{}) error {
	status := status.Newf(code, messageFormat, args...)
	cause := serializeErrorCause(err)
	status, newErr := status.WithDetails(cause)
	contract.AssertNoError(newErr)
	return status.Err()
}

// WithDetails adds arbitrary protobuf payloads to errors created by this package.
// These errors will be accessible by calling `Details` on `Error` instances created
// by `FromError`.
func WithDetails(err error, details ...proto.Message) error {
	status, ok := status.FromError(err)
	contract.Assertf(ok, "WithDetails called on error not created by rpcerror")
	status, conversionError := status.WithDetails(details...)
	contract.AssertNoError(conversionError)
	return status.Err()
}

// FromError "unwraps" an error created by functions in the `rpcerror` package and produces
// an `Error` structure from them.
//
// This function is designed to be used by clients interacting with gRPC servers.
// If the gRPC server issued an error using one of the error creation functions in `rpcerror`,
// this function will produce a non-null `Error`.
//
// Returns false if the given error is not a gRPC Status error.
func FromError(err error) (*Error, bool) {
	status, ok := status.FromError(err)
	if !ok {
		rpcError, ok := err.(*Error)
		return rpcError, ok
	}

	var rpcError Error
	rpcError.code = status.Code()
	rpcError.message = status.Message()
	rpcError.details = status.Details()
	for _, details := range status.Details() {
		if errorCause, ok := details.(*pulumirpc.ErrorCause); ok {
			contract.Assertf(rpcError.cause == nil, "RPC endpoint sent more than one ErrorCause")
			rpcError.cause = &ErrorCause{
				message:    errorCause.Message,
				stackTrace: errorCause.StackTrace,
			}
		}
	}

	return &rpcError, true
}

// Convert converts an error to an Error using `FromError`, but panics if the conversion
// fails.
func Convert(err error) *Error {
	converted, ok := FromError(err)
	contract.Assertf(ok, "failed to convert err %v to Error, did this come from an RPC endpoint?", err)
	return converted
}

func serializeErrorCause(err error) *pulumirpc.ErrorCause {
	// Go is a surprising language that lets you do wacky stuff like this
	// to get at implementation details of private structs.
	//
	// The pkg/errors documentation actually encourages this pattern (!) so
	// that's what we're doing here to get at the error's stack trace.
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}

	message := err.Error()
	var stackTrace string
	if errWithStack, ok := err.(stackTracer); ok {
		stackTrace = fmt.Sprintf("%+v", errWithStack.StackTrace())
	}

	return &pulumirpc.ErrorCause{
		Message:    message,
		StackTrace: stackTrace,
	}
}
