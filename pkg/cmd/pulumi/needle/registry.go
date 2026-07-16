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

package needle

import (
	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/spf13/cobra"
)

func RequireRegistry(registry *registry.Registry) Stitch {
	return request{
		value:       requireRegistry,
		fulfillInto: func(s *state) { *registry = s.registry },
	}
}

var requireRegistry = &value{
	deps: []*value{optionBackend},
	get: func(_ *cobra.Command, state *state, _ any) error {
		// When the user is not logged in maybeBackend leaves the backend nil, so fall back to the
		// unauthenticated registry, which can still resolve public packages.
		if state.backend != nil {
			state.registry = state.backend.GetReadOnlyCloudRegistry()
		} else {
			state.registry = unauthenticatedregistry.New(state.DiagSink, state.Env)
		}
		return nil
	},
}
