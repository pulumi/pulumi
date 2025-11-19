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

type stashArgs struct {
	Input any `pulumi:"input"`
}

type StashArgs struct {
	// The value to store in the stash resource.
	Input Input
}

func (StashArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*stashArgs)(nil)).Elem()
}

// NewStash creates a stash resource that stores a value
func NewStash(ctx *Context, name string, args *StashArgs,
	opts ...ResourceOption,
) (*Stash, error) {
	if args == nil {
		args = &StashArgs{}
	}
	stash := Stash{ctx: ctx}
	if err := ctx.RegisterResource("pulumi:index:Stash", name, args, &stash, opts...); err != nil {
		return nil, err
	}
	return &stash, nil
}
