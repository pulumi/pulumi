// Copyright 2016-2023, Pulumi Corporation.
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

package internal

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplier_Call(t *testing.T) {
	t.Parallel()

	stringType := reflect.TypeOf("")

	t.Run("minimal", func(t *testing.T) {
		t.Parallel()

		ap, err := newApplier(func(s string) int {
			assert.Equal(t, "hello", s)
			return 42
		}, stringType)
		require.NoError(t, err)

		o, err := ap.Call(context.Background(), reflect.ValueOf("hello"))
		require.NoError(t, err)
		assert.Equal(t, int64(42), o.Int())
	})

	t.Run("context", func(t *testing.T) {
		t.Parallel()

		giveCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ap, err := newApplier(func(ctx context.Context, s string) int {
			// == check because we want an exact reference match,
			// not a deep equals.
			assert.True(t, ctx == giveCtx, "context must match")
			assert.Equal(t, "hello", s)
			return 42
		}, stringType)
		require.NoError(t, err)

		o, err := ap.Call(giveCtx, reflect.ValueOf("hello"))
		require.NoError(t, err)
		assert.Equal(t, int64(42), o.Int())
	})

	t.Run("error/success", func(t *testing.T) {
		t.Parallel()

		ap, err := newApplier(func(string) (int, error) {
			return 42, nil
		}, stringType)
		require.NoError(t, err)

		o, err := ap.Call(context.Background(), reflect.ValueOf("hello"))
		require.NoError(t, err)
		assert.Equal(t, int64(42), o.Int())
	})

	t.Run("error/failure", func(t *testing.T) {
		t.Parallel()

		giveErr := errors.New("great sadness")

		ap, err := newApplier(func(string) (int, error) {
			return 0, giveErr
		}, stringType)
		require.NoError(t, err)

		_, err = ap.Call(context.Background(), reflect.ValueOf("hello"))
		// == check because we want an exact reference match,
		// not a deep equals.
		assert.True(t, err == giveErr, "error must match")
	})
}

func TestNewApplier_errors(t *testing.T) {
	t.Parallel()

	stringType := reflect.TypeOf("")
	tests := []struct {
		desc string
		give interface{}

		// Part of the error message expected in return.
		wantErr string
	}{
		{
			desc:    "not a function",
			give:    42,
			wantErr: "applier must be a function, got int",
		},
		{
			desc:    "no params",
			give:    func() int { return 0 },
			wantErr: "applier must accept exactly one or two parameters, got 0",
		},
		{
			desc:    "single param bad input",
			give:    func(int) int { return 0 },
			wantErr: "applier's first input parameter must be assignable from string, got int",
		},
		{
			desc:    "two params bad context",
			give:    func(int, string) int { return 0 },
			wantErr: "applier's first input parameter must be assignable from context.Context, got int",
		},
		{
			desc:    "two params bad input",
			give:    func(context.Context, int) int { return 0 },
			wantErr: "applier's second input parameter must be assignable from string, got int",
		},
		{
			desc:    "three params",
			give:    func(context.Context, string, int) int { return 0 },
			wantErr: "applier must accept exactly one or two parameters, got 3",
		},
		{
			desc:    "no returns",
			give:    func(string) {},
			wantErr: "applier must return exactly one or two values, got 0",
		},
		{
			desc:    "two returns bad error",
			give:    func(string) (int, int) { return 0, 0 },
			wantErr: "applier's second return type must be assignable to error, got int",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			_, err := newApplier(tt.give, stringType)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestOutputState_nil(t *testing.T) {
	t.Parallel()

	var os *OutputState

	assert.NotNil(t, os.elementType())
	assert.Empty(t, os.dependencies())

	// should be a no-op
	os.fulfillValue(reflect.ValueOf("hello"), true, true, nil /* deps */, nil /* err */)
}

func TestOutputWithDependencies(t *testing.T) {
	t.Parallel()

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	state := out.getState()
	require.Empty(t, state.dependencies())

	deps := []Resource{&ResourceState{}, &ResourceState{}}
	outputWithDeps := OutputWithDependencies(context.Background(), out, deps...)

	// The output with dependencies should be pending and track the dependencies
	stateWithDeps := outputWithDeps.getState()
	require.Equal(t, OutputPending, stateWithDeps.state)
	require.Equal(t, deps, stateWithDeps.dependencies())

	// Resolve the original output, which should also resolve the output with dependencies
	state.resolve(42, true, false, nil)

	v, known, secret, resolvedDeps, err := stateWithDeps.await(context.Background())
	require.NoError(t, err)
	require.Equal(t, 42, v)
	require.True(t, known)
	require.False(t, secret)
	require.Equal(t, deps, resolvedDeps)
}

func TestOutputWithDependenciesReject(t *testing.T) {
	t.Parallel()

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	state := out.getState()
	require.Empty(t, state.dependencies())

	deps := []Resource{&ResourceState{}, &ResourceState{}}
	outputWithDeps := OutputWithDependencies(context.Background(), out, deps...)

	// The output with dependencies should be pending and track the dependencies
	stateWithDeps := outputWithDeps.getState()
	require.Equal(t, OutputPending, stateWithDeps.state)
	require.Equal(t, deps, stateWithDeps.dependencies())

	// Reject the original output, which should also reject the output with dependencies
	state.reject(errors.New("oh no"))

	v, known, secret, resolvedDeps, err := stateWithDeps.await(context.Background())
	require.Error(t, err, "oh no")
	require.Nil(t, v)
	require.True(t, known)
	require.False(t, secret)
	require.Equal(t, deps, resolvedDeps)
}
