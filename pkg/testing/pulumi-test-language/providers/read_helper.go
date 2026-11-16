// Copyright 2026, Pulumi Corporation.
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

package providers

import (
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// coalesceImportOutputs returns the outputs a trivial mock provider should surface from an Import.
// Import carries no prior state, so this just echoes the identifying inputs as the live state.
func coalesceImportOutputs(req plugin.ImportRequest) *property.Map {
	return &req.Inputs
}

// coalesceRefreshOutputs returns the outputs a trivial mock provider should surface from a Refresh.
// Refresh carries prior outputs, which a trivial provider echoes back; if they're empty, fall back
// to the prior inputs.
func coalesceRefreshOutputs(req plugin.RefreshRequest) *property.Map {
	if req.Outputs.Len() > 0 {
		return &req.Outputs
	}
	return &req.Inputs
}
