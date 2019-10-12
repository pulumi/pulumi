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

package rpcerror

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestError(t *testing.T) {
	t.Parallel()
	err := New(codes.FailedPrecondition, "i failed a precondition")
	rpcErr, ok := FromError(err)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.Equal(t, codes.FailedPrecondition, rpcErr.Code())
	assert.Equal(t, "i failed a precondition", rpcErr.Error())
	assert.Nil(t, rpcErr.Cause())
}

func TestErrorf(t *testing.T) {
	t.Parallel()
	err := Newf(codes.AlreadyExists, "foo %d already exists", 42)

	rpcErr, ok := FromError(err)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.Equal(t, codes.AlreadyExists, rpcErr.Code())
	assert.Equal(t, "foo 42 already exists", rpcErr.Error())
	assert.Nil(t, rpcErr.Cause())
}

func TestWrap(t *testing.T) {
	t.Parallel()
	first := errors.New("first error")
	second := errors.Wrap(first, "second error")
	third := errors.Wrap(second, "third error")
	err := Wrap(codes.Aborted, third, "fourth error")

	rpcErr, ok := FromError(err)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.Equal(t, codes.Aborted, rpcErr.Code())
	assert.Equal(t, "fourth error", rpcErr.Message())
	if !assert.NotNil(t, rpcErr.Cause()) {
		t.FailNow()
	}

	cause := rpcErr.Cause()
	assert.Equal(t, "third error: second error: first error", cause.Message())

	// pkg/errors attaches stack traces to errors.New and errors.Wrap, so we
	// should have something.
	assert.NotEqual(t, "", cause.StackTrace())
}

func TestWrapWithoutBase(t *testing.T) {
	t.Parallel()
	wrapped := errors.New("first error")
	err := Wrap(codes.Unauthenticated, wrapped, "second error")

	rpcErr, ok := FromError(err)
	if !assert.True(t, ok) {
		t.FailNow()
	}

	assert.Equal(t, codes.Unauthenticated, rpcErr.Code())
	assert.Equal(t, "second error", rpcErr.Message())
	if !assert.NotNil(t, rpcErr.Cause()) {
		t.FailNow()
	}

	cause := rpcErr.Cause()
	assert.Equal(t, "first error", cause.Message())
	assert.NotEqual(t, "", cause.StackTrace())
}

func TestFromErrorRoundtrip(t *testing.T) {
	t.Parallel()
	status := status.New(codes.NotFound, "this is a raw gRPC error")

	// This scenario is exactly what RPC clients will see if they receive
	// an error off the wire
	rpcErr, ok := FromError(status.Err())
	if !assert.True(t, ok) || !assert.NotNil(t, rpcErr) {
		t.FailNow()
	}

	assert.Equal(t, codes.NotFound, rpcErr.Code())
	assert.Equal(t, "this is a raw gRPC error", rpcErr.Message())
	assert.Nil(t, rpcErr.Cause())

	// This is an elaborate no-op, but it's a different code path so it's still
	// useful to test
	rpcErrAgain, ok := FromError(rpcErr)
	if !assert.True(t, ok) || !assert.NotNil(t, rpcErr) {
		t.FailNow()
	}

	assert.Equal(t, codes.NotFound, rpcErrAgain.Code())
	assert.Equal(t, "this is a raw gRPC error", rpcErrAgain.Message())
	assert.Nil(t, rpcErrAgain.Cause())
}

func TestErrorString(t *testing.T) {
	t.Parallel()
	err := errors.New("oh no")
	withCause := Wrap(codes.NotFound, err, "thing failed")
	unwrapped, ok := FromError(withCause)
	if !assert.True(t, ok) || !assert.NotNil(t, unwrapped) {
		t.FailNow()
	}

	assert.Equal(t, "thing failed: oh no", unwrapped.Error())

	withoutCause := New(codes.NotFound, "thing failed 2")
	unwrapped, ok = FromError(withoutCause)
	if !assert.True(t, ok) || !assert.NotNil(t, unwrapped) {
		t.FailNow()
	}

	assert.Equal(t, "thing failed 2", unwrapped.Error())
}
