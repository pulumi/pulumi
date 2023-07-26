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

// ApplyContextErr applies a function to an input,
// returning an Output holding the result of the function.
//
// If the function returns an error, the Output will be in an error state.
func ApplyContextErr[A, B any](
	ctx context.Context,
	i Input[A],
	fn func(A) (B, error),
) Output[B] {
	o := i.ToOutput(ctx)

	var wg internal.WorkGroup
	state := internal.NewOutputState(&wg, typeOf[B](), internal.OutputDependencies(o)...)
	go func() {
		a, known, secret, deps, err := await(ctx, o)
		if err != nil || !known {
			var zero B
			internal.FulfillOutput(state, zero, known, secret, deps, err)
			return
		}

		b, err := fn(a)
		if err != nil {
			internal.RejectOutput(state, err)
			return
		}

		internal.FulfillOutput(state, b, known, secret, deps, nil)
	}()

	return Output[B]{OutputState: state}
}

// ApplyContext applies a function to an input,
// returning an Output holding the result of the function.
//
// This is a variant of ApplyContextErr
// that does not allow the function to return an error.
func ApplyContext[A, B any](
	ctx context.Context,
	i Input[A],
	fn func(A) B,
) Output[B] {
	return ApplyContextErr(
		ctx,
		i,
		func(a A) (B, error) {
			return fn(a), nil
		},
	)
}

// ApplyErr applies a function to an input,
// returning an Output holding the result of the function.
//
// If the function returns an error, the Output will be in an error state.
//
// This is a variant of ApplyContextErr
// that uses the background context.
func ApplyErr[A, B any](
	i Input[A],
	fn func(A) (B, error),
) Output[B] {
	ctx := context.Background()
	return ApplyContextErr(ctx, i, fn)
}

// Apply applies a function to an input,
// returning an Output holding the result of the function.
//
// This is a variant of ApplyContextErr
// that does not allow the function to return an error,
// and uses the background context.
func Apply[A, B any](
	i Input[A],
	fn func(A) B,
) Output[B] {
	ctx := context.Background()
	return ApplyContext(ctx, i, fn)
}
