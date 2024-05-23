package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

func TestCreateIfNotExists_WithImport(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(
					urn resource.URN,
					inputs resource.PropertyMap,
					timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "id/a", resource.PropertyMap{}, resource.StatusOK, nil
				},
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			ImportID:          "id/a",
			CreateIfNotExists: "id/a",
		})

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	snap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")

	assert.Nil(t, snap.VerifyIntegrity(), "Snapshot should be valid")
	assert.ErrorContains(t, err, "ImportId and CreateIfNotExists cannot be specified together")
}

func TestCreateIfNotExists_InProgram_NotInRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	createCalled := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(
					urn resource.URN,
					inputs resource.PropertyMap,
					timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					createCalled = true
					return "id/a", resource.PropertyMap{}, resource.StatusOK, nil
				},
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	snap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)

	assert.Nil(t, snap.VerifyIntegrity(), "Snapshot should be valid")

	assert.True(t, createCalled, "Create should be called")
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::a"), snap.Resources[1].URN)
}

func TestCreateIfNotExists_InProgram_InRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	createCalled := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(
					urn resource.URN,
					inputs resource.PropertyMap,
					timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					createCalled = true
					return "id/a", resource.PropertyMap{}, resource.StatusOK, nil
				},
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					if urn.Name() == "a" {
						return plugin.ReadResult{
							Inputs: resource.PropertyMap{
								"foo": resource.NewStringProperty("bar"),
							},
							Outputs: resource.PropertyMap{},
						}, resource.StatusOK, nil
					}

					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
			CreateIfNotExists: "id/a",
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	snap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)

	assert.Nil(t, snap.VerifyIntegrity(), "Snapshot should be valid")

	assert.False(t, createCalled, "Create should not be called")
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::a"), snap.Resources[1].URN)
	assert.Equal(t, "bar", snap.Resources[1].Inputs["foo"].StringValue())
}

func TestCreateIfNotExists_NotInProgram_NotDeletedWith_NotRetain_NotInRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    false,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity())
	assert.Len(t, beforeSnap.Resources, 2)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.True(t, deleteCalled, "Delete should be called")
	assert.Len(t, afterSnap.Resources, 0)
}

func TestCreateIfNotExists_NotInProgram_NotDeletedWith_NotRetain_InRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    false,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity(), "Snapshot should be valid")
	assert.Len(t, beforeSnap.Resources, 2)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					if urn.Name() == "a" {
						return plugin.ReadResult{
							Inputs:  resource.PropertyMap{},
							Outputs: resource.PropertyMap{},
						}, resource.StatusOK, nil
					}

					return plugin.ReadResult{}, resource.StatusOK, nil
				},
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.True(t, deleteCalled, "Delete should be called")
	assert.Len(t, afterSnap.Resources, 0)
}

func TestCreateIfNotExists_NotInProgram_NotDeletedWith_Retain_NotInRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    true,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity(), "Snapshot should be valid")
	assert.Len(t, beforeSnap.Resources, 2)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.False(t, deleteCalled, "Delete should not be called")
	assert.Len(t, afterSnap.Resources, 0)
}

func TestCreateIfNotExists_NotInProgram_NotDeletedWith_Retain_InRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    true,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity(), "Snapshot should be valid")
	assert.Len(t, beforeSnap.Resources, 2)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					if urn.Name() == "a" {
						return plugin.ReadResult{
							Inputs:  resource.PropertyMap{},
							Outputs: resource.PropertyMap{},
						}, resource.StatusOK, nil
					}

					return plugin.ReadResult{}, resource.StatusOK, nil
				},
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.False(t, deleteCalled, "Delete should not be called")
	assert.Len(t, afterSnap.Resources, 0)
}

func TestCreateIfNotExists_NotInProgram_DeletedWith_NotRetain_NotInRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dep", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    false,
			DeletedWith:       dep.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity(), "Snapshot should be valid")
	assert.Len(t, beforeSnap.Resources, 3)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.False(t, deleteCalled, "Delete should not be called")
	assert.Len(t, afterSnap.Resources, 0)
}

func TestCreateIfNotExists_NotInProgram_DeletedWith_NotRetain_InRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dep", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    false,
			DeletedWith:       dep.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity(), "Snapshot should be valid")
	assert.Len(t, beforeSnap.Resources, 3)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					if urn.Name() == "a" {
						return plugin.ReadResult{
							Inputs:  resource.PropertyMap{},
							Outputs: resource.PropertyMap{},
						}, resource.StatusOK, nil
					}

					return plugin.ReadResult{}, resource.StatusOK, nil
				},
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.False(t, deleteCalled, "Delete should not be called")
	assert.Len(t, afterSnap.Resources, 0)
}

func TestCreateIfNotExists_NotInProgram_DeletedWith_Retain_NotInRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dep", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    true,
			DeletedWith:       dep.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity(), "Snapshot should be valid")
	assert.Len(t, beforeSnap.Resources, 3)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.False(t, deleteCalled, "Delete should not be called")
	assert.Len(t, afterSnap.Resources, 0)
}

func TestCreateIfNotExists_NotInProgram_DeletedWith_Retain_InRemote(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}
	project := p.GetProject()

	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dep", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			CreateIfNotExists: "id/a",
			RetainOnDelete:    true,
			DeletedWith:       dep.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeProgramF, beforeLoaders...)
	beforeOptions := TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}

	beforeSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), beforeOptions, false, p.BackendClient, nil, "0")

	assert.NoError(t, err)
	assert.Nil(t, beforeSnap.VerifyIntegrity(), "Snapshot should be valid")
	assert.Len(t, beforeSnap.Resources, 3)

	deleteCalled := false
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					if urn.Name() == "a" {
						return plugin.ReadResult{
							Inputs:  resource.PropertyMap{},
							Outputs: resource.PropertyMap{},
						}, resource.StatusOK, nil
					}

					return plugin.ReadResult{}, resource.StatusOK, nil
				},
				DeleteF: func(
					urn resource.URN,
					_ resource.ID,
					_ resource.PropertyMap,
					_ resource.PropertyMap,
					_ float64,
				) (resource.Status, error) {
					if urn.Name() == "a" {
						deleteCalled = true
					}

					return resource.StatusOK, nil
				},
			}, nil
		}),
	}

	afterProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterProgramF, afterLoaders...)
	afterOptions := TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
	}

	afterSnap, err := TestOp(Update).
		RunStep(project, p.GetTarget(t, beforeSnap), afterOptions, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)

	assert.Nil(t, afterSnap.VerifyIntegrity(), "Snapshot should be valid")

	assert.False(t, deleteCalled, "Delete should not be called")
	assert.Len(t, afterSnap.Resources, 0)
}
