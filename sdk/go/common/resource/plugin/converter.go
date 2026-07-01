// Copyright 2016, Pulumi Corporation.
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

package plugin

import (
	"context"
	"io"

	"github.com/hashicorp/hcl/v2"

	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

type ResourceImport struct {
	Type        string
	Name        string
	ID          string
	LogicalName string
	IsComponent bool
	IsRemote    bool

	Version           string
	PluginDownloadURL string

	// Parameterization is set when the resource should be imported under a parameterized (e.g.
	// dynamically bridged) provider rather than a plain one.
	Parameterization *ResourceParameterization
}

// ResourceParameterization describes the base plugin that a resource's parameterized provider is built
// from. The parameterized package name and version are taken from the resource's own type and version.
type ResourceParameterization struct {
	// PluginName is the name of the base plugin to parameterize (e.g. "terraform-provider").
	PluginName string
	// PluginVersion is the version of the base plugin to parameterize.
	PluginVersion string
	// Value is the parameter value to apply to the base plugin.
	Value []byte
}

type ConvertStateRequest struct {
	MapperTarget string
	Args         []string
}

type ConvertStateResponse struct {
	Resources   []ResourceImport
	Diagnostics hcl.Diagnostics
}

type ConvertProgramRequest struct {
	SourceDirectory           string
	TargetDirectory           string
	MapperTarget              string
	LoaderTarget              string
	Args                      []string
	GeneratedProjectDirectory string
}

type ConvertProgramResponse struct {
	Diagnostics hcl.Diagnostics
}

type ConvertSnippetRequest struct {
	Filename     string
	Source       []byte
	TargetLoader string
	// Package identifies the package (and any parameterization) the snippet belongs to so the converter can load
	// the same schema we did when invoking the provider.
	Package    *codegenrpc.GetSchemaRequest
	Token      string
	Attributes map[string]string
}

type ConvertSnippetResponse struct {
	Diagnostics hcl.Diagnostics
	Filename    string
	Source      []byte
	Attributes  map[string]string
}

type Converter interface {
	io.Closer

	ConvertState(ctx context.Context, req *ConvertStateRequest) (*ConvertStateResponse, error)

	ConvertProgram(ctx context.Context, req *ConvertProgramRequest) (*ConvertProgramResponse, error)

	ConvertSnippet(ctx context.Context, req *ConvertSnippetRequest) (*ConvertSnippetResponse, error)
}
