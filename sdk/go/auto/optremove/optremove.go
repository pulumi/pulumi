// Copyright 2016-2025, Pulumi Corporation.
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

// Package optremove contains functional options to be used with workspace stack remove operations
// github.com/sdk/v2/go/x/auto workspace.RemoveStack(stackName, ...optremove.Option)
package optremove

// Force causes the remove operation to occur even if there are resources existing in the stack
func Force() Option {
	return optionFunc(func(opts *Options) {
		opts.Force = true
	})
}

// RemoveBackups indicates whether to remove backups of the stack, if using the DIY backend.
func RemoveBackups() Option {
	return optionFunc(func(opts *Options) {
		opts.RemoveBackups = true
	})
}

// Option is a parameter to be applied to a Stack.Remove() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// forces a stack to be deleted
	Force bool

	// RemoveBackups indicates whether to remove backups of the stack, if using the DIY backend.
	RemoveBackups bool
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
