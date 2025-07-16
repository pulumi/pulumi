// Copyright 2022-2024, Pulumi Corporation.
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
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDuplicateURN tests that duplicate URNs are disallowed.
func TestDuplicateURN(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)

		// Reads use the same URN namespace as register so make sure this also errors
		_, _, err = monitor.ReadResource("pkgA:m:typA", "resA", "id", "", resource.PropertyMap{}, "", "", "", "")
		assert.Error(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
}

// TestDuplicateAlias tests that multiple new resources may not claim to be aliases for the same old resource.
func TestDuplicateAlias(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	}

	runtimeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})
	hostF := deploytest.NewPluginHostF(nil, nil, runtimeF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		assert.Error(t, err)
		return nil
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.Error(t, err)
}

func TestSecretMasked(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// Return the secret value as an unmasked output. This should get masked by the engine.
					return plugin.CreateResponse{
						ID: "id",
						Properties: resource.PropertyMap{
							"shouldBeSecret": resource.NewStringProperty("bar"),
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"shouldBeSecret": resource.MakeSecret(resource.NewStringProperty("bar")),
			},
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	require.NotNil(t, snap)
	if snap != nil {
		assert.True(t, snap.Resources[1].Outputs["shouldBeSecret"].IsSecret())
	}
}

// TestReadReplaceStep creates a resource and then replaces it with a read resource.
func TestReadReplaceStep(t *testing.T) {
	t.Parallel()

	// Create resource.
	lt.NewTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
				return plugin.CreateResponse{
					ID:         "created-id",
					Properties: req.Properties,
					Status:     resource.StatusOK,
				}, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			require.NoError(t, err)
			return nil
		}, true).
		Then(func(snap *deploy.Snapshot, err error) {
			require.NoError(t, err)
			require.NotNil(t, snap)

			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.False(t, snap.Resources[1].External)

			// ReadReplace resource.
			lt.NewTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},
							Status:     resource.StatusOK,
						}, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "read-id", "", nil, "", "", "", "")
					require.NoError(t, err)
					return nil
				}, false).
				Then(func(snap *deploy.Snapshot, err error) {
					require.NoError(t, err)

					require.NotNil(t, snap)
					assert.Nil(t, snap.VerifyIntegrity())
					assert.Len(t, snap.Resources, 2)
					assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
					assert.True(t, snap.Resources[1].External)
				})
		})
}

func TestRelinquishStep(t *testing.T) {
	t.Parallel()

	const resourceID = "my-resource-id"
	lt.NewTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
				// Should match the ReadResource resource ID.
				return plugin.CreateResponse{
					ID:         resourceID,
					Properties: req.Properties,
					Status:     resource.StatusOK,
				}, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			require.NoError(t, err)
			return nil
		}, true).
		Then(func(snap *deploy.Snapshot, err error) {
			require.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.False(t, snap.Resources[1].External)

			lt.NewTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},
							Status:     resource.StatusOK,
						}, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", resourceID, "", nil, "", "", "", "")
					require.NoError(t, err)
					return nil
				}, true).
				Then(func(snap *deploy.Snapshot, err error) {
					require.NoError(t, err)

					require.NotNil(t, snap)
					assert.Nil(t, snap.VerifyIntegrity())
					assert.Len(t, snap.Resources, 2)
					assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
					assert.True(t, snap.Resources[1].External)
				})
		})
}

func TestTakeOwnershipStep(t *testing.T) {
	t.Parallel()

	lt.NewTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
				return plugin.ReadResponse{
					ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},
					Status:     resource.StatusOK,
				}, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "my-resource-id", "", nil, "", "", "", "")
			require.NoError(t, err)
			return nil
		}, false).
		Then(func(snap *deploy.Snapshot, err error) {
			require.NoError(t, err)

			require.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.True(t, snap.Resources[1].External)

			// Create new resource for this snapshot.
			lt.NewTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						// Should match the ReadF resource ID.
						return plugin.CreateResponse{
							ID:         "my-resource-id",
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
					require.NoError(t, err)
					return nil
				}, true).
				Then(func(snap *deploy.Snapshot, err error) {
					require.NoError(t, err)

					require.NotNil(t, snap)
					assert.Nil(t, snap.VerifyIntegrity())
					assert.Len(t, snap.Resources, 2)
					assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
					assert.False(t, snap.Resources[1].External)
				})
		})
}

func TestInitErrorsStep(t *testing.T) {
	t.Parallel()

	// Create new resource for this snapshot.
	lt.NewTestBuilder(t, &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   "pulumi:providers:pkgA",
				URN:    "urn:pulumi:test::test::pulumi:providers:pkgA::default",
				Custom: true,
				Delete: false,
				ID:     "935b2216-aec5-4810-96fd-5f6eae57ac88",
			},
			{
				Type:     "pkgA:m:typA",
				URN:      "urn:pulumi:test::test::pkgA:m:typA::resA",
				Custom:   true,
				ID:       "my-resource-id",
				Provider: "urn:pulumi:test::test::pulumi:providers:pkgA::default::935b2216-aec5-4810-96fd-5f6eae57ac88",
				InitErrors: []string{
					`errors should yield an empty update to "continue" awaiting initialization.`,
				},
			},
		},
	}).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
				return plugin.CreateResponse{
					ID:         "my-resource-id",
					Properties: req.Properties,
					Status:     resource.StatusOK,
				}, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			require.NoError(t, err)
			return nil
		}, false).
		Then(func(snap *deploy.Snapshot, err error) {
			require.NoError(t, err)

			require.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.Empty(t, snap.Resources[1].InitErrors)
		})
}

func TestReadNilOutputs(t *testing.T) {
	t.Parallel()

	const resourceID = "my-resource-id"
	lt.NewTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
				return plugin.ReadResponse{}, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", resourceID, "", nil, "", "", "", "")
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")

			return nil
		}, true).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.ErrorContains(t, err,
				"BAIL: step executor errored: step application failed: resource 'my-resource-id' does not exist")
		})
}
