// Copyright 2016-2026, Pulumi Corporation.
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
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	deployProviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// TestEnvVarMappingsOnProvider tests that envVarMappings can be set on provider resources
// and is stored in the provider's Inputs.
func TestEnvVarMappingsOnProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		require.NoError(t, err)

		// Register a provider with envVarMappings
		_, err = monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{
			EnvVarMappings: map[string]string{
				"MY_VAR":    "PROVIDER_VAR",
				"OTHER_VAR": "TARGET_VAR",
			},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Verify the snapshot has the provider with envVarMappings in Inputs
	require.Len(t, snap.Resources, 2) // stack + provider
	provRes := snap.Resources[1]
	require.True(t, providers.IsProviderType(provRes.Type))
	assert.Equal(t, "prov", provRes.URN.Name())

	// Check that envVarMappings is in the provider's Inputs under __internal
	envVarMappings, err := deployProviders.GetEnvironmentVariableMappings(provRes.Inputs)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"MY_VAR":    "PROVIDER_VAR",
		"OTHER_VAR": "TARGET_VAR",
	}, envVarMappings)
}

// TestEnvVarMappingsOnNonProviderFails tests that envVarMappings cannot be used on
// non-provider resources and returns an error.
func TestEnvVarMappingsOnNonProviderFails(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		require.NoError(t, err)

		// Try to register a non-provider resource with envVarMappings - this should fail
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			EnvVarMappings: map[string]string{
				"MY_VAR": "PROVIDER_VAR",
			},
		})
		// The error should be returned here
		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	_, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "envVarMappings can only be used with provider resources")
}
