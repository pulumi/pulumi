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

// Package optup contains functional options to be used with stack updates
// github.com/sdk/v2/go/x/auto Stack.Up(...optup.Option)
package optup

import (
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
)

// Parallel is the number of resource operations to run in parallel at once during the update
// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
func Parallel(n int) Option {
	return optionFunc(func(opts *Options) {
		opts.Parallel = n
	})
}

// Message (optional) to associate with the update operation
func Message(message string) Option {
	return optionFunc(func(opts *Options) {
		opts.Message = message
	})
}

// ExpectNoChanges will cause the update to return an error if any changes occur
func ExpectNoChanges() Option {
	return optionFunc(func(opts *Options) {
		opts.ExpectNoChanges = true
	})
}

// Diff displays operation as a rich diff showing the overall change
func Diff() Option {
	return optionFunc(func(opts *Options) {
		opts.Diff = true
	})
}

// Replace specifies an array of resource URNs to explicitly replace during the update
func Replace(urns []string) Option {
	return optionFunc(func(opts *Options) {
		opts.Replace = urns
	})
}

// Target specifies an exclusive list of resource URNs to update
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

// ProgressStreams allows specifying one or more io.Writers to redirect incremental update output
func ProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ProgressStreams = writers
	})
}

// DebugLogging provides options for verbose logging to standard error, and enabling plugin logs.
func DebugLogging(debugOpts debug.LoggingOptions) Option {
	return optionFunc(func(opts *Options) {
		opts.DebugLogOpts = debugOpts
	})
}

// EventStreams allows specifying one or more channels to receive the Pulumi event stream
func EventStreams(channels ...chan<- events.EngineEvent) Option {
	return optionFunc(func(opts *Options) {
		opts.EventStreams = channels
	})
}

// UserAgent specifies the agent responsible for the update, stored in backends as "environment.exec.agent"
func UserAgent(agent string) Option {
	return optionFunc(func(opts *Options) {
		opts.UserAgent = agent
	})
}

// Option is a parameter to be applied to a Stack.Up() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// Parallel is the number of resource operations to run in parallel at once
	// (1 for no parallelism). Defaults to unbounded. (default 2147483647)
	Parallel int
	// Message (optional) to associate with the update operation
	Message string
	// Return an error if any changes occur during this update
	ExpectNoChanges bool
	// Diff displays operation as a rich diff showing the overall change
	Diff bool
	// Specify resources to replace
	Replace []string
	// Specify an exclusive list of resource URNs to update
	Target []string
	// Allows updating of dependent targets discovered but not specified in the Target list
	TargetDependents bool
	// DebugLogOpts specifies additional settings for debug logging
	DebugLogOpts debug.LoggingOptions
	// ProgressStreams allows specifying one or more io.Writers to redirect incremental update output
	ProgressStreams []io.Writer
	// EventStreams allows specifying one or more channels to receive the Pulumi event stream
	EventStreams []chan<- events.EngineEvent
	// UserAgent specifies the agent responsible for the update, stored in backends as "environment.exec.agent"
	UserAgent string
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
