package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
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
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)

		// Reads use the same URN namespace as register so make sure this also errors
		_, _, err = monitor.ReadResource("pkgA:m:typA", "resA", "id", "", resource.PropertyMap{}, "", "", "")
		assert.Error(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	_, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
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
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	}

	runtimeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})
	hostF := deploytest.NewPluginHostF(nil, nil, runtimeF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		assert.Error(t, err)
		return nil
	}

	_, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
}

func TestSecretMasked(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					// Return the secret value as an unmasked output. This should get masked by the engine.
					return "id", resource.PropertyMap{
						"shouldBeSecret": resource.NewStringProperty("bar"),
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"shouldBeSecret": resource.MakeSecret(resource.NewStringProperty("bar")),
			},
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.NotNil(t, snap)
	if snap != nil {
		assert.True(t, snap.Resources[1].Outputs["shouldBeSecret"].IsSecret())
	}
}

// TestReadReplaceStep creates a resource and then replaces it with a read resource.
func TestReadReplaceStep(t *testing.T) {
	t.Parallel()

	// Create resource.
	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64, preview bool,
			) (resource.ID, resource.PropertyMap, resource.Status, error) {
				return "created-id", news, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.NoError(t, err)
			return nil
		}).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NoError(t, err)
			assert.NotNil(t, snap)

			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.False(t, snap.Resources[1].External)

			// ReadReplace resource.
			newTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					ReadF: func(urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
					) (plugin.ReadResult, resource.Status, error) {
						return plugin.ReadResult{Outputs: resource.PropertyMap{}}, resource.StatusOK, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "read-id", "", nil, "", "", "")
					assert.NoError(t, err)
					return nil
				}).
				Then(func(snap *deploy.Snapshot, err error) {
					assert.NoError(t, err)

					assert.NotNil(t, snap)
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
	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
				preview bool,
			) (resource.ID, resource.PropertyMap, resource.Status, error) {
				// Should match the ReadResource resource ID.
				return resourceID, news, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.NoError(t, err)
			return nil
		}).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.False(t, snap.Resources[1].External)

			newTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					ReadF: func(urn resource.URN, id resource.ID,
						inputs, state resource.PropertyMap,
					) (plugin.ReadResult, resource.Status, error) {
						return plugin.ReadResult{
							Outputs: resource.PropertyMap{},
						}, resource.StatusOK, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", resourceID, "", nil, "", "", "")
					assert.NoError(t, err)
					return nil
				}).
				Then(func(snap *deploy.Snapshot, err error) {
					assert.NoError(t, err)

					assert.NotNil(t, snap)
					assert.Nil(t, snap.VerifyIntegrity())
					assert.Len(t, snap.Resources, 2)
					assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
					assert.True(t, snap.Resources[1].External)
				})
		})
}

func TestTakeOwnershipStep(t *testing.T) {
	t.Parallel()

	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			ReadF: func(urn resource.URN, id resource.ID,
				inputs, state resource.PropertyMap,
			) (plugin.ReadResult, resource.Status, error) {
				return plugin.ReadResult{
					Outputs: resource.PropertyMap{},
				}, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "my-resource-id", "", nil, "", "", "")
			assert.NoError(t, err)
			return nil
		}).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NoError(t, err)

			assert.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.True(t, snap.Resources[1].External)

			// Create new resource for this snapshot.
			newTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
						preview bool,
					) (resource.ID, resource.PropertyMap, resource.Status, error) {
						// Should match the ReadF resource ID.
						return "my-resource-id", news, resource.StatusOK, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
					assert.NoError(t, err)
					return nil
				}).
				Then(func(snap *deploy.Snapshot, err error) {
					assert.NoError(t, err)

					assert.NotNil(t, snap)
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
	newTestBuilder(t, &deploy.Snapshot{
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
			CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
				preview bool,
			) (resource.ID, resource.PropertyMap, resource.Status, error) {
				return "my-resource-id", news, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.NoError(t, err)
			return nil
		}).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NoError(t, err)

			assert.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.Empty(t, snap.Resources[1].InitErrors)
		})
}

func TestReadNilOutputs(t *testing.T) {
	t.Parallel()

	const resourceID = "my-resource-id"
	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			ReadF: func(urn resource.URN, id resource.ID,
				inputs, state resource.PropertyMap,
			) (plugin.ReadResult, resource.Status, error) {
				return plugin.ReadResult{}, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", resourceID, "", nil, "", "", "")
			assert.ErrorContains(t, err, "resource 'my-resource-id' does not exist")

			return nil
		}).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.ErrorContains(t, err,
				"BAIL: step executor errored: step application failed: resource 'my-resource-id' does not exist")
		})
}
