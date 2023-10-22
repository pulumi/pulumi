// Copyright 2016-2023, Pulumi Corporation.
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
)

type ResourceImport struct {
	Type string
	Name string
	ID   string

	Version           string
	PluginDownloadURL string
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
	SourceDirectory string
	TargetDirectory string
	MapperTarget    string
	LoaderTarget    string
	Args            []string
}

type ConvertProgramResponse struct {
	Diagnostics hcl.Diagnostics
}

type Converter interface {
	io.Closer

	ConvertState(ctx context.Context, req *ConvertStateRequest) (*ConvertStateResponse, error)

	ConvertProgram(ctx context.Context, req *ConvertProgramRequest) (*ConvertProgramResponse, error)
}
