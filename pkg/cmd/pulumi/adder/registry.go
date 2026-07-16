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

package adder

import (
	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/spf13/cobra"
)

// Registry returns the package registry. Resolution — including the backend
// lookup behind it — is deferred until the registry is first used, so commands
// that never query it never touch credentials.
func (s Spindle) Registry(cmd *cobra.Command) registry.Registry {
	return registry.NewOnDemandRegistry(func() (registry.Registry, error) {
		return bagFrom(cmd).registry.get(func() (registry.Registry, error) {
			b, err := s.CurrentBackend(cmd)
			if err != nil {
				return nil, err
			}
			if b != nil {
				return b.GetReadOnlyCloudRegistry(), nil
			}
			// Not logged in: fall back to the unauthenticated registry, which
			// can still resolve public packages.
			s := s.defaults(cmd)
			return unauthenticatedregistry.New(s.DiagSink, s.Env), nil
		})
	})
}
