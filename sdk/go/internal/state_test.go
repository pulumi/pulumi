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

type intOutput struct {
	*OutputState
}

func (o intOutput) ElementType() reflect.Type { return reflect.TypeOf(0) }

func TestRejectOutput(t *testing.T) {
	t.Parallel()

	giveErr := errors.New("great sadness")

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	assert.Equal(t, OutputPending, GetOutputStatus(out))
	RejectOutput(out, giveErr)

	_, _, _, _, err := AwaitOutput(context.Background(), out)
	assert.ErrorIs(t, err, giveErr)
	assert.Equal(t, OutputRejected, GetOutputStatus(out))
}

func TestResolveOutput(t *testing.T) {
	t.Parallel()

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	assert.Equal(t, OutputPending, GetOutputStatus(out))
	ResolveOutput(out, 42, true, false, nil)

	got, known, secret, deps, err := AwaitOutput(context.Background(), out)
	require.NoError(t, err)
	assert.Equal(t, 42, got)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Empty(t, deps)
	assert.Equal(t, OutputResolved, GetOutputStatus(out))
}

func TestResolveOutput_alreadyResolved(t *testing.T) {
	t.Parallel()

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	ResolveOutput(out, 42, true, false, nil)
	assert.Equal(t, OutputResolved, GetOutputStatus(out))

	ResolveOutput(out, 43, true, false, nil)
	got, _, _, _, err := AwaitOutput(context.Background(), out)
	require.NoError(t, err)
	assert.Equal(t, 42, got)
}

func TestFulfillOutput_success(t *testing.T) {
	t.Parallel()

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	FulfillOutput(out, 42, true, false, nil, nil)

	got, known, secret, deps, err := AwaitOutput(context.Background(), out)
	require.NoError(t, err)
	assert.Equal(t, 42, got)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Empty(t, deps)
}

func TestFulfillOutput_error(t *testing.T) {
	t.Parallel()

	giveErr := errors.New("great sadness")

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	FulfillOutput(out, 42, true, false, nil, giveErr)

	_, _, _, _, err := AwaitOutput(context.Background(), out)
	assert.ErrorIs(t, err, giveErr)
}

func TestOutputDependencies(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out := NewOutput(nil, reflect.TypeOf(intOutput{}))
		assert.Empty(t, OutputDependencies(out))
	})

	t.Run("nonempty", func(t *testing.T) {
		t.Parallel()

		deps := []Resource{&ResourceState{}, &ResourceState{}}

		out := NewOutput(nil, reflect.TypeOf(intOutput{}))
		ResolveOutput(out, 42, true, false, deps)

		gotDeps := OutputDependencies(out)
		assert.Len(t, gotDeps, 2)
		for i, dep := range deps {
			assert.Same(t, dep, gotDeps[i])
		}
	})
}

func TestGetOutputState(t *testing.T) {
	t.Parallel()

	state := &OutputState{}
	o := intOutput{OutputState: state}
	got := GetOutputState(o)
	assert.Same(t, state, got)
}

func TestGetOutputValue(t *testing.T) {
	t.Parallel()

	o := NewOutput(nil, reflect.TypeOf(intOutput{}))
	assert.Nil(t, GetOutputValue(o))

	ResolveOutput(o, 42, true, false, nil)
	assert.Equal(t, 42, GetOutputValue(o))
}

func TestAwaitOutput_contextExpired(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := NewOutput(nil, reflect.TypeOf(intOutput{}))
	_, _, _, _, err := AwaitOutput(ctx, out)

	assert.ErrorIs(t, err, context.Canceled)
}
