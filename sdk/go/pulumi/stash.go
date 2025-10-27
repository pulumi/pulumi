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

// Stash manages a reference to a Pulumi stash resource.
type Stash struct {
	CustomResourceState

	// Value is any value stored in the stash resource
	Value AnyOutput `pulumi:"value"`

	// ctx is a reference to the context used to create the state. It must be
	// valid and non-nil to call `GetOutput`.
	ctx *Context
}

type stashArgs struct {
	Value       any   `pulumi:"value"`
	Passthrough *bool `pulumi:"passthrough"`
}

type StashArgs struct {
	Value       Input
	Passthrough BoolPtrInput
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
	if err := ctx.RegisterResource("pulumi:pulumi:Stash", name, args, &stash, opts...); err != nil {
		return nil, err
	}
	return &stash, nil
}
