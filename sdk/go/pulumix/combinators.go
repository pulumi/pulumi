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
// It awwaits the given output, stores the value in the given pointer,
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

// JoinContext unpacks an Output stored inside another input,
// returning an output containing the underlying value.
func JoinContext[A any, I Input[A]](ctx context.Context, i Input[I]) Output[A] {
	outputOutputA := i.ToOutput(ctx)
	stateOutputOutputA := internal.GetOutputState(outputOutputA)

	stateA := internal.NewOutputState(
		internal.OutputJoinGroup(stateOutputOutputA),
		typeOf[A](),
		internal.OutputDependencies(stateOutputOutputA)...,
	)
	go func() {
		var outputA I
		var a A

		applier := newApplyNState[A](stateA)
		applyNStep(ctx, &applier, outputOutputA, &outputA)
		applyNStep(ctx, &applier, outputA.ToOutput(ctx), &a)

		if applier.ok {
			applier.finish(a, nil /* err */)
		}
	}()

	return Output[A]{OutputState: stateA}
}

// Join unpacks the Output stored inside another input,
// returning an output containing the underlying value.
//
// This is a variant of JoinContext
// that uses the background context.
func Join[A any, I Input[A]](i Input[I]) Output[A] {
	return JoinContext[A, I](context.Background(), i)
}

// All combines multiple inputs into a single output
// that produces a list of all the input values.
func All(args ...Input[any]) Output[[]any] {
	return Array[any](args).ToOutput(context.Background())
}
