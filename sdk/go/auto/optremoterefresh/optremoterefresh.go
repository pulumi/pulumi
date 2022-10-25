// Copyright 2016-2022, Pulumi Corporation.
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

// Package optremoterefresh contains functional options to be used with remote stack refresh operations
// github.com/sdk/v3/go/auto RemoteStack.Refresh(...optremoterefresh.Option)
package optremoterefresh

import (
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
)

// ProgressStreams allows specifying one or more io.Writers to redirect incremental refresh stdout
func ProgressStreams(writers ...io.Writer) Option {
	return optionFunc(func(opts *Options) {
		opts.ProgressStreams = writers
	})
}

// ErrorProgressStreams allows specifying one or more io.Writers to redirect incremental refresh stderr
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

// Option is a parameter to be applied to a Stack.Refresh() operation
type Option interface {
	ApplyOption(*Options)
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// ProgressStreams allows specifying one or more io.Writers to redirect incremental refresh stdout
	ProgressStreams []io.Writer
	// ErrorProgressStreams allows specifying one or more io.Writers to redirect incremental refresh stderr
	ErrorProgressStreams []io.Writer
	// EventStreams allows specifying one or more channels to receive the Pulumi event stream
	EventStreams []chan<- events.EngineEvent
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
