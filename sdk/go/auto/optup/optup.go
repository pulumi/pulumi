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

// Exclude specifies an exclusive list of resource URNs to ignore
func Exclude(urns []string) Option {
	return optionFunc(func(opts *Options) {
		opts.Exclude = urns
	})
}

// ExcludeDependents allows ignoring of dependent targets discovered but not specified in the Exclude list
func ExcludeDependents() Option {
	return optionFunc(func(opts *Options) {
		opts.ExcludeDependents = true
	})
}

// ProgressStreams allows specifying one or more io.Writers to redirect incremental update stdout
func ProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ProgressStreams = writers
	})
}

// ErrorProgressStreams allows specifying one or more io.Writers to redirect incremental update stderr
func ErrorProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ErrorProgressStreams = writers
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

// Color allows specifying whether to colorize output. Choices are: always, never, raw, auto (default "auto")
func Color(color string) Option {
	return optionFunc(func(opts *Options) {
		opts.Color = color
	})
}

// Plan specifies the path to an update plan to use for the update.
func Plan(path string) Option {
	return optionFunc(func(opts *Options) {
		opts.Plan = path
	})
}

// ShowSecrets configures whether to show config secrets when they appear.
func ShowSecrets(show bool) Option {
	return optionFunc(func(opts *Options) {
		opts.ShowSecrets = &show
	})
}

// Suppress display of periodic progress dots
func SuppressProgress() Option {
	return optionFunc(func(opts *Options) {
		opts.SuppressProgress = true
	})
}

// Suppress display of stack outputs (in case they contain sensitive values)
func SuppressOutputs() Option {
	return optionFunc(func(opts *Options) {
		opts.SuppressOutputs = true
	})
}

// Refresh will refresh the stack's state before the update.
func Refresh() Option {
	return optionFunc(func(opts *Options) {
		opts.Refresh = true
	})
}

// ContinueOnError will continue to perform the update operation despite the occurrence of errors.
func ContinueOnError() Option {
	return optionFunc(func(opts *Options) {
		opts.ContinueOnError = true
	})
}

// AttachDebugger will run the process under a debugger, and pause until a debugger is attached
func AttachDebugger() Option {
	return optionFunc(func(opts *Options) {
		opts.AttachDebugger = true
	})
}

// ConfigFile specifies a file to use for configuration values rather than detecting the file name
func ConfigFile(path string) Option {
	return optionFunc(func(opts *Options) {
		opts.ConfigFile = path
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
	// Specify an exclusive of resource URNs to ignore
	Exclude []string
	// Allows ignoring of dependent targets discovered but not specified in the Exclude list
	ExcludeDependents bool
	// DebugLogOpts specifies additional settings for debug logging
	DebugLogOpts debug.LoggingOptions
	// ProgressStreams allows specifying one or more io.Writers to redirect incremental update stdout
	ProgressStreams []io.Writer
	// ErrorProgressStreams allows specifying one or more io.Writers to redirect incremental update stderr
	ErrorProgressStreams []io.Writer
	// EventStreams allows specifying one or more channels to receive the Pulumi event stream
	EventStreams []chan<- events.EngineEvent
	// UserAgent specifies the agent responsible for the update, stored in backends as "environment.exec.agent"
	UserAgent string
	// Colorize output. Choices are: always, never, raw, auto (default "auto")
	Color string
	// Use the update plan at the given path.
	Plan string
	// Run one or more policy packs as part of this update
	PolicyPacks []string
	// Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag
	PolicyPackConfigs []string
	// Show config secrets when they appear.
	ShowSecrets *bool
	// Refresh will refresh the stack's state before the update.
	Refresh bool
	// Suppress display of periodic progress dots
	SuppressProgress bool
	// Suppress display of stack outputs (in case they contain sensitive values)
	SuppressOutputs bool
	// ContinueOnError will continue to perform the update operation despite the occurrence of errors.
	ContinueOnError bool
	// AttachDebugger will run the process under a debugger, and pause until a debugger is attached
	AttachDebugger bool
	// Run using the configuration values in the specified file rather than detecting the file name
	ConfigFile string
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
