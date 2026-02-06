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

// TestEmptyEnvVarMappingsNotStoredInState tests that when a provider is created without
// envVarMappings (or with empty mappings), the envVarMappings key is not stored in state.
func TestEmptyEnvVarMappingsNotStoredInState(t *testing.T) {
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

		// Register a provider WITHOUT envVarMappings
		_, err = monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
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

	// Verify the snapshot has the provider
	require.Len(t, snap.Resources, 2) // stack + provider
	provRes := snap.Resources[1]
	require.True(t, providers.IsProviderType(provRes.Type))
	assert.Equal(t, "prov", provRes.URN.Name())

	// Check that envVarMappings is NOT in the provider's Inputs
	// The __internal key may or may not exist, but if it does, envVarMappings should not be there
	internal, hasInternal := provRes.Inputs["__internal"]
	if hasInternal {
		internalMap := internal.ObjectValue()
		_, hasEnvVarMappings := internalMap["envVarMappings"]
		assert.False(t, hasEnvVarMappings, "envVarMappings should not be stored when empty")
	}
	// If __internal doesn't exist, that's also fine - no envVarMappings
}

// TestEnvVarMappingsRemovedFromStateOnUpdate tests that when a provider initially has
// envVarMappings and is then updated to not have them, the key is removed from state.
func TestEnvVarMappingsRemovedFromStateOnUpdate(t *testing.T) {
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
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					// Report a diff if inputs changed
					if !req.OldInputs.DeepEquals(req.NewInputs) {
						return plugin.DiffResponse{
							Changes: plugin.DiffSome,
						}, nil
					}
					return plugin.DiffResponse{Changes: plugin.DiffNone}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	// Track which run we're on
	runNumber := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		require.NoError(t, err)

		runNumber++
		if runNumber == 1 {
			// First run: register provider WITH envVarMappings
			_, err = monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{
				EnvVarMappings: map[string]string{
					"MY_VAR": "PROVIDER_VAR",
				},
			})
		} else {
			// Second run: register provider WITHOUT envVarMappings
			_, err = monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		}
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	// First update: provider has envVarMappings
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Verify envVarMappings is present after first update
	require.Len(t, snap.Resources, 2)
	provRes := snap.Resources[1]
	require.True(t, providers.IsProviderType(provRes.Type))
	envVarMappings, err := deployProviders.GetEnvironmentVariableMappings(provRes.Inputs)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"MY_VAR": "PROVIDER_VAR"}, envVarMappings)

	// Second update: provider no longer has envVarMappings
	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// Verify envVarMappings is NOT present after second update
	require.Len(t, snap.Resources, 2)
	provRes = snap.Resources[1]
	require.True(t, providers.IsProviderType(provRes.Type))

	internal, hasInternal := provRes.Inputs["__internal"]
	if hasInternal {
		internalMap := internal.ObjectValue()
		_, hasEnvVarMappings := internalMap["envVarMappings"]
		assert.False(t, hasEnvVarMappings, "envVarMappings should be removed when no longer specified")
	}
}
