// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package rpcerrors

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RPCError represents an error response from an RPC server endpoint.
// It contains a gRPC error code, a message, and a chain of "wrapped"
// errors that led to the final dispatch of this particular error message.
type RPCError struct {
	code    codes.Code
	message string
	cause   *RPCErrorCause
}

func (r *RPCError) Error() string {
	if r.cause != nil {
		return fmt.Sprintf("%s: %s", r.message, r.cause.Message())
	}

	return r.message
}

// Code returns the gRPC error code for this error.
func (r *RPCError) Code() codes.Code {
	return r.code
}

// Message returns the message associated with this error cause.
func (r *RPCError) Message() string {
	return r.message
}

// Cause returns the error that was the root cause of this error,
// or nil if one wasn't provided.
func (r *RPCError) Cause() *RPCErrorCause {
	return r.cause
}

// RPCErrorCause represents a root cause of an error that ultimately caused
// an RPC endpoint to issue an error. RPCErrorCauses are optionally attached
// to RPCErrors.
//
// All RPCErrorCauses have messages, but only a subset of them have stack traces.
// Notably, the pkg/errors package will affix stack traces to errors created through
// the errors.New and errors.Wrap.
type RPCErrorCause struct {
	message    string
	stackTrace string
}

// Message returns the message associated with this error cause.
func (r *RPCErrorCause) Message() string {
	return r.message
}

// StackTrace returns the stack trace associated with this error, or
// the empty string if one wasn't provided.
func (r *RPCErrorCause) StackTrace() string {
	return r.stackTrace
}

// Error creates a new gRPC-compatible `error` with the given error code
// and message.
func Error(code codes.Code, message string) error {
	status := status.New(code, message)
	return status.Err()
}

// Errorf creates a new gRPC-compatible `error` with the given code and
// formatted message.
func Errorf(code codes.Code, messageFormat string, args ...interface{}) error {
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
	status := status.Newf(code, messageFormat, args)
	cause := serializeErrorCause(err)
	status, newErr := status.WithDetails(cause)
	contract.AssertNoError(newErr)
	return status.Err()
}

// FromError "unwraps" an error created by functions in the `rpcerror` package and produces
// an `RPCError` structure from them.
//
// This function is designed to be used by clients interacting with gRPC servers.
// If the gRPC server issued an error using one of the error creation functions in `rpcerror`,
// this function will produce a non-null `RPCError`.
//
// Returns false if the given error is not a gRPC Status error.
func FromError(err error) (*RPCError, bool) {
	status, ok := status.FromError(err)
	if !ok {
		rpcError, ok := err.(*RPCError)
		return rpcError, ok
	}

	var rpcError RPCError
	rpcError.code = status.Code()
	rpcError.message = status.Message()
	for _, details := range status.Details() {
		if errorCause, ok := details.(*pulumirpc.ErrorCause); ok {
			contract.Assertf(rpcError.cause == nil, "RPC endpoint sent more than one ErrorCause")
			rpcError.cause = &RPCErrorCause{
				message:    errorCause.Message,
				stackTrace: errorCause.StackTrace,
			}
		}
	}

	return &rpcError, true
}

// Convert converts an error to an RPCError using `FromError`, but panics if the conversion
// fails.
func Convert(err error) *RPCError {
	converted, ok := FromError(err)
	contract.Assertf(ok, "failed to convert err %v to RPCError, did this come from an RPC endpoint?", err)
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
