// Copyright 2025, Pulumi Corporation.
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

package convert

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ProviderFactory functions accept a PackageDescriptor and return a Provider. If the PackageDescriptor specifies a
// parameterization, the factory is responsible for returning a provider that has already been appropriately
// parameterized.
type ProviderFactory func(descriptor workspace.PackageDescriptor) (plugin.Provider, error)

// ProviderFactoryFromHost builds a ProviderFactory that uses the given plugin host to create providers and manage their
// lifecycles.
func ProviderFactoryFromHost(ctx context.Context, host plugin.Host) ProviderFactory {
	return func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		if descriptor.Kind != apitype.ResourcePlugin {
			return nil, fmt.Errorf("provider factory must be a resource plugin package, was %v", descriptor.Kind)
		}

		provider, err := host.Provider(descriptor)
		if err != nil {
			desc := descriptor.PackageName()
			v := descriptor.PackageVersion()
			if v != nil {
				desc += "@" + v.String()
			}
			return nil, fmt.Errorf("load plugin %v: %w", desc, err)
		}

		if descriptor.Parameterization != nil {
			_, err := provider.Parameterize(ctx, plugin.ParameterizeRequest{
				Parameters: &plugin.ParameterizeValue{
					Name:    descriptor.Parameterization.Name,
					Version: descriptor.Parameterization.Version,
					Value:   descriptor.Parameterization.Value,
				},
			})
			if err != nil {
				return nil, fmt.Errorf("failed to parameterize provider %q: %w", descriptor.PackageName(), err)
			}
		}

		return &hostManagedProvider{
			Provider: provider,
			host:     host,
		}, nil
	}
}

// hostManagedProvider wraps a Provider such that it can be closed by the host that created it.
type hostManagedProvider struct {
	plugin.Provider

	host plugin.Host
}

var _ plugin.Provider = (*hostManagedProvider)(nil)

// Overrides the wrapped provider's implementation of Provider.Close to ask the managing plugin host to close the
// provider.
func (pc *hostManagedProvider) Close() error {
	return pc.host.CloseProvider(pc.Provider)
}
