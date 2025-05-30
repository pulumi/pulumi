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
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
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

// ProgressStreams allows specifying one or more io.Writers to redirect incremental destroy stdout
func ProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ProgressStreams = writers
	})
}

// ErrorProgressStreams allows specifying one or more io.Writers to redirect incremental destroy stderr
func ErrorProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ErrorProgressStreams = writers
	})
}

// EventStreams allows specifying one or more channels to receive the Pulumi event stream
func EventStreams(channels ...chan<- events.EngineEvent) Option {
	return optionFunc(func(opts *Options) {
		opts.EventStreams = channels
	})
}

// DebugLogging provides options for verbose logging to standard error, and enabling plugin logs.
func DebugLogging(debugOpts debug.LoggingOptions) Option {
	return optionFunc(func(opts *Options) {
		opts.DebugLogOpts = debugOpts
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

// ShowSecrets configures whether to show config secrets when they appear.
func ShowSecrets(show bool) Option {
	return optionFunc(func(opts *Options) {
		opts.ShowSecrets = &show
	})
}

// Refresh will run a refresh before the destroy.
func Refresh() Option {
	return optionFunc(func(opts *Options) {
		opts.Refresh = true
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

// Continue to perform the destroy operation despite the occurrence of errors.
func ContinueOnError() Option {
	return optionFunc(func(opts *Options) {
		opts.ContinueOnError = true
	})
}

// Remove the stack and its configuration after all resources in the stack have
// been deleted.
func Remove() Option {
	return optionFunc(func(opts *Options) {
		opts.Remove = true
	})
}

// ConfigFile specifies a file to use for configuration values rather than detecting the file name
func ConfigFile(path string) Option {
	return optionFunc(func(opts *Options) {
		opts.ConfigFile = path
	})
}

// RunProgram runs the program in the workspace to perform the destroy.
func RunProgram(f bool) Option {
	return optionFunc(func(opts *Options) {
		opts.RunProgram = &f
	})
}

// SkipPreview(true) disables calculating a preview before performing the destroy.
func SkipPreview(doSkipPreview bool) Option {
	return optionFunc(func(opts *Options) {
		opts.SkipPreview = doSkipPreview
	})
}

// PreviewOnly(true) only shows a preview of the destroy, but does not perform the destroy itself.
func PreviewOnly(previewDestroy bool) Option {
	return optionFunc(func(opts *Options) {
		opts.PreviewOnly = previewDestroy
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
	// Specify an exclusive list of resource URNs to ignore
	Exclude []string
	// Allows ignoring of dependent targets discovered but not specified in the Exclude list
	ExcludeDependents bool
	// ProgressStreams allows specifying one or more io.Writers to redirect incremental destroy stdout
	ProgressStreams []io.Writer
	// ProgressStreams allows specifying one or more io.Writers to redirect incremental destroy stderr
	ErrorProgressStreams []io.Writer
	// EventStreams allows specifying one or more channels to receive the Pulumi event stream
	EventStreams []chan<- events.EngineEvent
	// DebugLogOpts specifies additional settings for debug logging
	DebugLogOpts debug.LoggingOptions
	// UserAgent specifies the agent responsible for the update, stored in backends as "environment.exec.agent"
	UserAgent string
	// Colorize output. Choices are: always, never, raw, auto (default "auto")
	Color string
	// Show config secrets when they appear.
	ShowSecrets *bool
	// Refresh will run a refresh before the destroy.
	Refresh bool
	// Suppress display of periodic progress dots
	SuppressProgress bool
	// Suppress display of stack outputs (in case they contain sensitive values)
	SuppressOutputs bool
	// Continue to perform the destroy operation despite the occurrence of errors.
	ContinueOnError bool
	// Remove the stack and its configuration after all resources in the stack
	// have been deleted.
	Remove bool
	// Run using the configuration values in the specified file rather than detecting the file name
	ConfigFile string
	// When set to true, run the program in the workspace to perform the destroy.
	RunProgram *bool
	// Disable calculating a preview before performing the destroy.
	SkipPreview bool
	// Only show a preview of the destroy, but does not perform the destroy itself.
	PreviewOnly bool
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
