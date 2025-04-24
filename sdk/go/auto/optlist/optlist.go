// Copyright 2016-2024, Pulumi Corporation.
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

// Package optlist contains functional options to be used with workspace stack list operations
// github.com/sdk/v3/go/auto workspace.ListStacks(context.Context, ...optlist.Option)
package optlist

func All() Option {
	return optionFunc(func(opts *Options) {
		opts.All = true
	})
}

// Option is a parameter to be applied to a LocalWorkspace.ListStacks() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------
// Options is an implementation detail
type Options struct {
	// lists all stacks
	All bool
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
