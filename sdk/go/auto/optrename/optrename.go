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

// Package optrename contains functional options to be used with stack rename operations
// github.com/sdk/v2/go/x/auto Stack.Rename(...optrename.Option)
package optrename

import "io"

// The new name of the stack.
func StackName(message string) Option {
	return optionFunc(func(opts *Options) {
		opts.StackName = message
	})
}

// ErrorProgressStreams allows specifying one or more io.Writers to redirect incremental rename stderr
func ErrorProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ErrorProgressStreams = writers
	})
}

// ShowSecrets configures whether to show config secrets when they appear in the config.
func ShowSecrets(show bool) Option {
	return optionFunc(func(opts *Options) {
		opts.ShowSecrets = &show
	})
}

// Option is a parameter to be applied to a Stack.Rename() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// The new name for the stack.
	StackName string
	// ProgressStreams allows specifying one or more io.Writers to redirect incremental refresh stdout
	ProgressStreams []io.Writer
	// ErrorProgressStreams allows specifying one or more io.Writers to redirect incremental refresh stderr
	ErrorProgressStreams []io.Writer
	// Show config secrets when they appear.
	ShowSecrets *bool
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
