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

// Package opthistory contains functional options to be used with stack history operations
// github.com/sdk/v3/go/x/auto Stack.History(ctx, pageSize, page, ...opthistory.Option)
package opthistory

// ShowSecrets configures whether to show config secrets when they appear.
func ShowSecrets(show bool) Option {
	return optionFunc(func(opts *Options) {
		opts.ShowSecrets = &show
	})
}

// ---------------------------------- implementation details ----------------------------------

// Options is an implementation detail
type Options struct {
	// Show config secrets when they appear.
	ShowSecrets *bool
}

// Option is a parameter to be applied to a Stack.History() operation
type Option interface {
	ApplyOption(*Options)
}

type optionFunc func(*Options)

// ApplyOption is an implementation detail
func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
