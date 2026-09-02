// Copyright 2025, Pulumi Corporation.
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

package pulumi

import (
	"fmt"
	"reflect"
)

// Stash stores an arbitrary value in the state.
type Stash struct {
	CustomResourceState

	// The value saved in the state for the stash.
	Output AnyOutput `pulumi:"output"`

	// The most recent value passed to the stash resource.
	Input AnyOutput `pulumi:"input"`

	// ctx is a reference to the context used to create the state. It must be
	// valid and non-nil to call `GetOutput`.
	ctx *Context
}

type stashReducerRef struct {
	Target string `pulumi:"target"`
	Token  string `pulumi:"token"`
}

type stashArgs struct {
	Input   any              `pulumi:"input"`
	Reducer *stashReducerRef `pulumi:"reducer"`
}

type StashArgs struct {
	// The value to store in the stash resource.
	Input Input
	// An optional reducer combining the previously stashed input and output with the current
	// program input to produce a new output. On create the reducer is skipped and the initial
	// output is just the input.
	Reduce StashReducerFunction
}

func (StashArgs) ElementType() reflect.Type {
	return reflect.TypeFor[stashArgs]()
}

// NewStash creates a stash resource that stores a value
func NewStash(ctx *Context, name string, args *StashArgs,
	opts ...ResourceOption,
) (*Stash, error) {
	if args == nil {
		args = &StashArgs{}
	}

	// If a reducer callback was supplied, register it with the callback server and translate
	// the args into a form the engine understands. The builtin provider invokes the callback
	// during Check on update; the reducer object itself is never persisted in state.
	var internalArgs Input = args
	if args.Reduce != nil {
		cb, err := ctx.registerStashReducer(args.Reduce)
		if err != nil {
			return nil, fmt.Errorf("registering stash reducer: %w", err)
		}
		internalArgs = &stashInternalArgs{
			Input: args.Input,
			Reducer: &stashReducerRef{
				Target: cb.Target,
				Token:  cb.Token,
			},
		}
	}

	stash := Stash{ctx: ctx}
	if err := ctx.RegisterResource("pulumi:index:Stash", name, internalArgs, &stash, opts...); err != nil {
		return nil, err
	}
	return &stash, nil
}

// stashInternalArgs is the shape sent to the engine when a reducer callback is present.
type stashInternalArgs struct {
	Input   Input
	Reducer *stashReducerRef
}

func (stashInternalArgs) ElementType() reflect.Type {
	return reflect.TypeFor[stashArgs]()
}
