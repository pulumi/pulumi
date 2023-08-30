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

import "context"

// JoinContext unpacks an Output stored inside another input,
// returning an output containing the underlying value.
func JoinContext[A any, I Input[A]](ctx context.Context, inputInputA Input[I]) Output[A] {
	return Compose[A](ctx, func(c *Composer) (A, error) {
		inputA := ComposeAwait(c, inputInputA)
		a := ComposeAwait[A](c, inputA)
		return a, nil
	})
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
