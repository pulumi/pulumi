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

// Package optimport contains functional options to be used with workspace stack import operations
// github.com/sdk/v3/go/auto workspace.Import(context.Context, ...optimport.Option)
package optimport

import "io"

// Option is a parameter to be applied to a LocalWorkspace.Import() operation
type Option interface {
	ApplyOption(*Options)
}

// Protect configures whether to set imported resources as protected in the Pulumi state.
func Protect(value bool) Option {
	return optionFunc(func(opts *Options) {
		opts.Protect = &value
	})
}

// GenerateCode configures whether to generate code for the imported resources
func GenerateCode(value bool) Option {
	return optionFunc(func(opts *Options) {
		opts.GenerateCode = &value
	})
}

// NameTable maps language names to parent and provider URNs. These names are
// used in the generated definitions, and should match the corresponding declarations
// in the source program. This table is required if any parents or providers are
// specified by the resources to import.
func NameTable(nameTable map[string]string) Option {
	return optionFunc(func(opts *Options) {
		opts.NameTable = nameTable
	})
}

// Resources specified the resources to import
func Resources(resources []*ImportResource) Option {
	return optionFunc(func(opts *Options) {
		opts.Resources = resources
	})
}

// Converter specifies the converter to use for importing the resources
func Converter(converter string) Option {
	return optionFunc(func(opts *Options) {
		opts.Converter = &converter
	})
}

// ConverterArgs specifies the arguments to pass to the converter used for the import
func ConverterArgs(converterArgs []string) Option {
	return optionFunc(func(opts *Options) {
		opts.ConverterArgs = converterArgs
	})
}

// ShowSecrets configures whether to show config secrets when they appear.
func ShowSecrets(value bool) Option {
	return optionFunc(func(opts *Options) {
		opts.ShowSecrets = value
	})
}

// Message to associate with the update operation
func Message(message string) Option {
	return optionFunc(func(opts *Options) {
		opts.Message = message
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

// PreviewOnly allows preview the import without executing the import
func PreviewOnly(previewOnly bool) Option {
	return optionFunc(func(opts *Options) {
		opts.PreviewOnly = &previewOnly
	})
}

// ImportFile specifies the file to import resources from
func ImportFile(importFile string) Option {
	return optionFunc(func(opts *Options) {
		opts.ImportFile = &importFile
	})
}

// Diff specifies whether to show the diff of the import
func Diff(diff bool) Option {
	return optionFunc(func(opts *Options) {
		opts.Diff = &diff
	})
}

type ImportResource struct {
	// The ID of the resource to import. The format of the ID is determined by the resource's provider.
	ID string `json:"id,omitempty"`
	// The type token of the Pulumi resource
	Type string `json:"type,omitempty"`
	// The name of the resource
	Name string `json:"name,omitempty"`
	// The name of the resource used in the generated Pulumi program from the import
	LogicalName string `json:"logicalName,omitempty"`
	// The parent of the resource
	Parent string `json:"parent,omitempty"`
	// The provider to use for importing the resource
	Provider string `json:"provider,omitempty"`
	// The version of the provider of the resource
	Version string `json:"version,omitempty"`
	// The URL to download the provider plugin from
	PluginDownloadURL string `json:"pluginDownloadUrl,omitempty"`
	// The input properties to use when importing the resource
	Properties []string `json:"properties,omitempty"`
	// Whether the resource is a component placeholder. When specifying this option, you don't need to provide an ID.
	Component bool `json:"component,omitempty"`
	// When the resource is a component, this specifies it as a remote component.
	Remote bool `json:"remote,omitempty"`
}

type Options struct {
	Protect              *bool
	GenerateCode         *bool
	NameTable            map[string]string
	Resources            []*ImportResource
	Converter            *string
	ConverterArgs        []string
	ShowSecrets          bool
	Message              string
	ProgressStreams      []io.Writer
	ErrorProgressStreams []io.Writer
	PreviewOnly          *bool
	ImportFile           *string
	Diff                 *bool
}

type optionFunc func(*Options)

func (o optionFunc) ApplyOption(opts *Options) {
	o(opts)
}
