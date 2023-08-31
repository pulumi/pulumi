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
	"errors"
	"runtime"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
)

// C allows composing multiple outputs together in a type-safe manner.
//
// To use C, begin a composition with the [Compose] function.
// This returns an Output[O] that will be fulfilled with the value returned by f.
//
//	o := pulumix.Compose(ctx, func(c *pulumix.C) (O, error) {
//		// ...
//	})
//
// Inside the function, use [CAwait] to await other outputs.
//
//	v1 := pulumix.CAwait(c, o1)
//	v2 := pulumix.CAwait(c, o2)
//	// ...
//
// Combine the values into a single result and return it.
// For example:
//
//	return &MyStruct{
//		Foo: v1,
//		Bar: v2,
//		...
//	}, nil
//
// If f returns an error, the output will be rejected.
// If any of the awaited outputs fail, the composition will abort
// and the output will be rejected.
//
// C allows combining an arbitrary number of outputs.
// For simpler cases, consider [Apply] or any of the ApplyN variants.
type C struct {
	ctx         context.Context
	known       bool
	secret      bool
	deps        []internal.Resource
	outputState *internal.OutputState

	fulfilled bool // true if the output has been fulfilled
}

// Compose begins a new output composition operation.
//
//	o := pulumix.Compose[O](ctx, func(c *pulumix.C) (O, error) {
//		// ...
//	})
//
// Inside f, use [CAwait] to await other outputs.
// The value returned by f is the value of the output.
// If f returns an error, the output will be rejected.
//
// Compose has some restrictions:
//
//   - f MUST call CAwait in the same goroutine.
//   - f MUST NOT use the C after it returns.
//   - f SHOULD NOT spawn new goroutines.
//     If it does, it MUST NOT use the C in those goroutines.
func Compose[T any](ctx context.Context, f func(*C) (T, error)) Output[T] {
	var wg internal.WorkGroup
	outputState := internal.NewOutputState(&wg, typeOf[T]())
	c := C{
		ctx:         ctx,
		known:       true,
		secret:      false,
		outputState: outputState,
	}

	go func() {
		defer func() {
			// If f kills this goroutine before returning,
			// it was because of one of two reasons:
			//
			//  - CAwait was called with an unknown or failed input
			//    which killed the goroutine but fulfilled the output state.
			//  - The user killed the goroutine with a panic
			//    or by calling runtime.Goexit().
			//
			// For the latter case, to avoid a deadlock
			// we must fulfill the output state before exiting.
			// We'll do this by filling it with a sentinel error,
			// although this wil not be seen by the user
			// because the panic will take precedence.
			if !c.fulfilled {
				err := errors.New("goroutine exited before returning: " +
					"this was likely caused by a panic")
				internal.RejectOutput(outputState, err)
			}

			// After the function returns, zero out the C
			// to protect against misuse like storing a pointer
			// to the C outside f.
			// e.g.,
			//
			//	var c *C
			//	o := Compose(ctx, func(c2 *C) (O, error) {
			//		c = *c2
			//		// ...
			//	})
			c = C{}
		}()

		v, err := f(&c)
		if err != nil {
			internal.RejectOutput(outputState, err)
		} else {
			internal.FulfillOutput(outputState, v, c.known, c.secret, c.deps, nil)
		}
	}()

	return Output[T]{OutputState: outputState}
}

// CAwait awaits for the output of the given input and returns it.
//
//	var o pulumix.Output[T] = someOutput
//	v := pulumix.CAwait(c, o)
//	// v is of type T
//
// Use this to combine multiple outputs into a single value.
//
//	i, err := strconv.ParseInt(pulumix.CAwait(c, strOutput))
//	if err != nil {
//		return 0, err
//	}
//
// If the input is unknown or failed,
// CAwait will cancel the entire composition operation.
// For example, given the following,
//
//	v1 := pulumix.CAwait(c, o1)
//	v2 := pulumix.CAwait(c, o2)
//	return f(v1, v2)
//
// If either o1 or o2 is unknown or failed,
// the composition will abort and f will not be called.
//
// In using CAwait, be aware that:
//
//   - It can only be called inside a [Compose] call.
//   - It MUST NOT be called from goroutines spawned by f.
func CAwait[T any](c *C, o Input[T]) T {
	contract.Assertf(c.outputState != nil, "CAwait called outside Compose")

	v, known, secret, deps, err := await(c.ctx, o.ToOutput(c.ctx))
	c.secret = c.secret || secret
	c.known = c.known && known
	c.deps = append(c.deps, deps...)
	if err != nil || !known {
		var zero T
		internal.FulfillOutput(c.outputState, zero, false, c.secret, c.deps, err)
		c.fulfilled = true
		runtime.Goexit()
	}

	return v
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
