// Copyright 2016-2020, Pulumi Corporation.
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

// Package optdestroy contains functional options to be used with stack destroy operations
// github.com/sdk/v2/go/x/auto Stack.Destroy(...optdestroy.Option)
package optdestroy

import (
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/debug"
	"io"
)

// Parallel is the number of resource operations to run in parallel at once during the destroy
// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
func Parallel(n int) Option {
	return optionFunc(func(opts *Options) {
		opts.Parallel = n
	})
}

// Message (optional) to associate with the destroy operation
func Message(message string) Option {
	return optionFunc(func(opts *Options) {
		opts.Message = message
	})
}

// Target specifies an exclusive list of resource URNs to destroy
func Target(urns []string) Option {
	return optionFunc(func(opts *Options) {
		opts.Target = urns
	})
}

// TargetDependents allows updating of dependent targets discovered but not specified in the Target list
func TargetDependents() Option {
	return optionFunc(func(opts *Options) {
		opts.TargetDependents = true
	})
}

// ProgressStreams allows specifying one or more io.Writers to redirect incremental destroy output
func ProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ProgressStreams = writers
	})
}

func DebugLogging(debugOpts debug.LoggingOptions) Option {
	return optionFunc(func(opts *Options) {
		opts.DebugLogOpts = debugOpts
	})
}

// Option is a parameter to be applied to a Stack.Destroy() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// Parallel is the number of resource operations to run in parallel at once
	// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
	Parallel int
	// Message (optional) to associate with the destroy operation
	Message string
	// Specify an exclusive list of resource URNs to update
	Target []string
	// Allows updating of dependent targets discovered but not specified in the Target list
	TargetDependents bool
	// ProgressStreams allows specifying one or more io.Writers to redirect incremental destroy output
	ProgressStreams []io.Writer
	// DebugLogOpts specifies additional settings for debug logging
	DebugLogOpts debug.LoggingOptions
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
