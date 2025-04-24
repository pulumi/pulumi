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
	"reflect"
)

// OutputOrState is satisfied by both Output and OutputState.
// It allows writing functions that can accept either type.
type OutputOrState interface {
	getState() *OutputState
}

var (
	_ OutputOrState = Output(nil)
	_ OutputOrState = (*OutputState)(nil)
)

// RejectOutput rejects the given output with the given error.
func RejectOutput(o OutputOrState, err error) {
	o.getState().reject(err)
}

// ResolveOutput resolves the given output with the given value and dependencies.
func ResolveOutput(o OutputOrState, value interface{}, known, secret bool, deps []Resource) {
	o.getState().resolve(value, known, secret, deps)
}

// AwaitOutput awaits the given output and returns the resulting state.
func AwaitOutput(ctx context.Context, o OutputOrState) (
	value interface{}, known, secret bool, deps []Resource, err error,
) {
	return o.getState().await(ctx)
}

// AwaitOutputNoUnwrap awaits the given output and returns the resulting state.
// Unlike [AwaitOutput], this does not unwrap the output value.
//
// That is, given an 'Output<Output<string>>', this will return 'Output<string>',
// while [AwaitOutput] would return 'string'.
func AwaitOutputNoUnwrap(ctx context.Context, o OutputOrState) (interface{}, bool, bool, []Resource, error) {
	return o.getState().awaitWithOptions(ctx, false /* unwrapOutputs */)
}

// FulfillOutput fulfills the given output with the given value and dependencies,
// or rejects it with the given error.
func FulfillOutput(o OutputOrState, value interface{}, known, secret bool, deps []Resource, err error) {
	o.getState().fulfill(value, known, secret, deps, err)
}

// OutputDependencies returns the dependencies of the given output.
func OutputDependencies(o OutputOrState) []Resource {
	return o.getState().dependencies()
}

// GetOutputState returns the OutputState for the given output.
func GetOutputState(o OutputOrState) *OutputState {
	return o.getState()
}

// OutputJoinGroup returns the WorkGroup for the given output.
// Use this when constructing new connected outputs.
func OutputJoinGroup(o OutputOrState) *WorkGroup {
	return o.getState().join
}

// ConcreteTypeToOutputType maps the given concrete type
// to the corresponding output type.
// The returned type implements Output.
//
// For example, this maps 'string' to 'StringOutput'.
func ConcreteTypeToOutputType(t reflect.Type) reflect.Type {
	if ot, ok := concreteTypeToOutputType.Load(t); ok {
		return ot.(reflect.Type)
	}
	return nil
}

// InputInterfaceTypeToConcreteType maps the given input interface type
// to the corresponding concrete type.
//
// For example, this maps 'URNInput' to 'URN'.
func InputInterfaceTypeToConcreteType(t reflect.Type) reflect.Type {
	if ct, ok := inputInterfaceTypeToConcreteType.Load(t); ok {
		return ct.(reflect.Type)
	}
	return nil
}

// GetOutputStatus reports the status of the given Output.
// This is one of Pending, Resolved, or Rejected.
func GetOutputStatus(o OutputOrState) OutputStatus {
	return o.getState().state
}

// GetOutputValue returns the value currently held in the Output.
//
// If the Output has not yet resolved (see [GetOutputStatus], it will return nil.
func GetOutputValue(o OutputOrState) interface{} {
	return o.getState().value
}
