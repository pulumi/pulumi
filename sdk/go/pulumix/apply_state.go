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

package pulumix

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
)

// applyNState helps implement the ApplyN combinators
// without excessive code duplication.
// O is the final type of the output.
//
// Intended usage:
//
//	// Start a new ApplyN operation.
//	st := newApplyNState[O](outputState)
//
//	// For each Output[A], declare a variable for the value (A),
//	// and call applyNStep. It will await, fill the value,
//	// and update internal state as needed.
//	var a A
//	applyNStep(ctx, &st, a, &a)
//	...
//
//	// Once all the steps are done, consume the recorded values
//	// and call st.finish -- if the applyn hasn't already failed.
//	if st.ok {
//		st.finish(f(a, b, c, ...))
//	}
type applyNState[O any] struct {
	ok   bool
	zero O

	known       bool
	secret      bool
	deps        []internal.Resource
	outputState *internal.OutputState
}

func newApplyNState[O any](outputState *internal.OutputState) applyNState[O] {
	return applyNState[O]{
		ok:          true,
		known:       true,
		secret:      false,
		outputState: outputState,
	}
}

// applyNStep takes a single step in an ApplyN computation.
// It awaits the given output, stores the value in the given pointer,
// and updates the internal state accordingly.
//
// If the applyN has already failed, this is a no-op.
func applyNStep[A, O any](ctx context.Context, st *applyNState[O], o Output[A], dst *A) {
	if !st.ok {
		return
	}

	v, known, secret, deps, err := await(ctx, o)
	st.secret = st.secret || secret
	st.known = st.known && known
	st.deps = append(st.deps, deps...)
	if err != nil || !known {
		st.ok = false
		internal.FulfillOutput(st.outputState, st.zero, false, st.secret, st.deps, err)
		return
	}

	*dst = v
}

// finish finishes the applyN computation.
// Call it with the result of the function passed to Apply.
func (st *applyNState[O]) finish(v O, err error) {
	if err != nil {
		internal.RejectOutput(st.outputState, err)
	} else {
		internal.FulfillOutput(st.outputState, v, st.known, st.secret, st.deps, nil)
	}
}

// await is a type-safe variant of OutputState.await.
//
// It disables unwrapping of nested Output values.
// Otherwise, await `Output[Output[T]]` would return `T`, not `Output[T]`,
// which will then panic.
func await[T any](ctx context.Context, o Output[T]) (value T, known, secret bool, deps []internal.Resource, err error) {
	iface, known, secret, deps, err := internal.AwaitOutputNoUnwrap(ctx, o)
	if known && err == nil {
		var ok bool
		value, ok = iface.(T)
		contract.Assertf(ok, "await expected %v, got %T", typeOf[T](), iface)
	}
	return value, known, secret, deps, err
}
