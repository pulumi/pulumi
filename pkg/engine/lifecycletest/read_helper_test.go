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

package lifecycletest

import (
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// coalesceReadOutputs returns the outputs a trivial mock provider should surface from a Read.
// On refresh the engine passes prior outputs in req.State; on a user-driven Get the engine
// passes an empty map, so fall back to req.Inputs to emulate a provider that returns the live state.
func coalesceReadOutputs(req plugin.ReadRequest) *property.Map {
	if req.State.Len() > 0 {
		s := req.State
		return &s
	}
	i := req.Inputs
	return &i
}
