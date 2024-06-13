// Copyright 2024-2024, Pulumi Corporation.
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
	"errors"
	"slices"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestroyContinueOnError(t *testing.T) {
	t.Parallel()
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap,
					timeout float64,
				) (resource.Status, error) {
					return resource.StatusOK, errors.New("intentionally failed delete")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	createResource := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			resp, err := monitor.RegisterResource("pkgA:m:typA", "unrelated1", true, deploytest.ResourceOptions{})
			assert.NoError(t, err)
			_, err = monitor.RegisterResource("pkgA:m:typA", "unrelated2", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{resp.URN},
			})
			assert.NoError(t, err)

			resp, err = monitor.RegisterResource("pkgA:m:typA", "dependency", true, deploytest.ResourceOptions{})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgB:m:typB", "failing", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{resp.URN},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "anotherUnrelatedRes", true, deploytest.ResourceOptions{})
			assert.NoError(t, err)

		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T: t,
			// Skip display tests because different ordering makes the colouring different.
			SkipDisplayTests: true,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
			},
			HostF: hostF,
		},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 7) // We expect 5 resources + 2 providers

	createResource = false
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "intentionally failed delete")
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4) // We expect 2 resources + 2 providers
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgA::default"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::dependency"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgB::default"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgB:m:typB::failing"), snap.Resources[3].URN)
}

func TestUpContinueOnErrorCreate(t *testing.T) {
	t.Parallel()

	// createFGenerator generates a create function that returns a different parameters on each call.
	// This generator supports creating up to 3 resources and allows us to test all the different code paths within
	// the create step's Apply method, which was overlooked and caused https://github.com/pulumi/pulumi/issues/16373.
	createFGenerator := func() func() (resource.ID, resource.PropertyMap, resource.Status, error) {
		counter := 0
		return func() (resource.ID, resource.PropertyMap, resource.Status, error) {
			counter++

			switch counter {
			case 1:
				// Return a non-StatusPartialFailure status.
				return "", nil, resource.StatusOK, errors.New("intentionally failed create")
			case 2:
				// Return a StatusPartialFailure status with an empty ID.
				return "", nil, resource.StatusPartialFailure, errors.New("intentionally failed create")
			default:
				// Return a StatusPartialFailure status with a non-empty ID.
				return "fakeid", nil, resource.StatusPartialFailure, errors.New("intentionally failed create")
			}
		}
	}
	createF := createFGenerator()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return createF()
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		failingResp, err := monitor.RegisterResource("pkgB:m:typB", "failing", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_FAIL, failingResp.Result)

		failingResp2, err := monitor.RegisterResource("pkgB:m:typB", "failing2", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_FAIL, failingResp2.Result)

		failingResp3, err := monitor.RegisterResource("pkgB:m:typB", "failing3", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_FAIL, failingResp3.Result)

		respIndependent1, err := monitor.RegisterResource(
			"pkgA:m:typA", "independent1", true, deploytest.ResourceOptions{SupportsResultReporting: true})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent1.Result)

		respIndependent2, err := monitor.RegisterResource(
			"pkgA:m:typA", "independent2", true, deploytest.ResourceOptions{
				SupportsResultReporting: true,
				Dependencies:            []resource.URN{respIndependent1.URN},
			})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent2.Result)

		respIndependent3, err := monitor.RegisterResource("pkgA:m:typA", "independent3", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Dependencies:            []resource.URN{respIndependent2.URN},
		})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent3.Result)

		respDepOnFailing, err := monitor.RegisterResource(
			"pkgA:m:typA", "dependentOnFailing", true, deploytest.ResourceOptions{
				SupportsResultReporting: true,
				Dependencies:            []resource.URN{failingResp.URN},
			})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SKIP, respDepOnFailing.Result)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
			},
			HostF:            hostF,
			SkipDisplayTests: true,
		},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, "intentionally failed create")
	require.NotNil(t, snap)

	expectedURNs := []string{
		"urn:pulumi:test::test::pulumi:providers:pkgB::default",
		"urn:pulumi:test::test::pulumi:providers:pkgA::default",
		"urn:pulumi:test::test::pkgA:m:typA::independent1",
		"urn:pulumi:test::test::pkgA:m:typA::independent2",
		"urn:pulumi:test::test::pkgA:m:typA::independent3",
		"urn:pulumi:test::test::pkgB:m:typB::failing2",
		"urn:pulumi:test::test::pkgB:m:typB::failing3",
	}

	assert.Equal(t, len(expectedURNs), len(snap.Resources))

	for _, urn := range expectedURNs {
		// Ensure that the expected URN is present in the snapshot.
		found := slices.ContainsFunc(snap.Resources, func(rs *resource.State) bool {
			return rs.URN == resource.URN(urn)
		})
		assert.True(t, found, "Expected URN %s not found in snapshot", urn)
	}
}

func TestUpContinueOnErrorUpdate(t *testing.T) {
	t.Parallel()

	// statusGenerator generates a status that is OK on the first call and PartialFailure
	// on the second call. This is so we can test when UpdateF returns different statuses,
	// while using the same provider. This allows us to test all the different code paths
	// the step's Apply method, which was overlooked
	// and caused https://github.com/pulumi/pulumi/issues/16373.
	statusGenerator := func() func() resource.Status {
		counter := 0
		return func() resource.Status {
			counter++
			if counter == 1 {
				return resource.StatusOK
			}
			return resource.StatusPartialFailure
		}
	}
	status := statusGenerator()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				UpdateF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap,
					timeout float64, ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					return nil, status(), errors.New("intentionally failed update")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	// update as opposed to create for the failing resource
	update := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgB:m:typB", "failing", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  ins,
		})
		assert.NoError(t, err)
		if update {
			assert.Equal(t, pulumirpc.Result_FAIL, resp.Result)
		} else {
			// On creation we expect to succeed
			assert.Equal(t, pulumirpc.Result_SUCCESS, resp.Result)
		}

		resp2, err := monitor.RegisterResource("pkgB:m:typB", "failing2", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  ins,
		})
		assert.NoError(t, err)
		if update {
			assert.Equal(t, pulumirpc.Result_FAIL, resp2.Result)
		} else {
			// On creation we expect to succeed
			assert.Equal(t, pulumirpc.Result_SUCCESS, resp2.Result)
		}

		if update {
			respIndependent1, err := monitor.RegisterResource(
				"pkgA:m:typA", "independent1", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
				})
			assert.NoError(t, err)
			assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent1.Result)

			respIndependent2, err := monitor.RegisterResource(
				"pkgA:m:typA", "independent2", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
					Dependencies:            []resource.URN{respIndependent1.URN},
				})
			assert.NoError(t, err)
			assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent2.Result)

			respIndependent3, err := monitor.RegisterResource("pkgA:m:typA", "independent3", true, deploytest.ResourceOptions{
				SupportsResultReporting: true,
				Dependencies:            []resource.URN{respIndependent2.URN},
			})
			assert.NoError(t, err)
			assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent3.Result)
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
			},
			HostF:            hostF,
			SkipDisplayTests: true,
		},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Equal(t, 3, len(snap.Resources)) // 2 resources + 1 provider

	update = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	// Run an update to create the resource
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "intentionally failed update")
	assert.NotNil(t, snap)
	expectedURNs := []string{
		"urn:pulumi:test::test::pulumi:providers:pkgB::default",
		"urn:pulumi:test::test::pulumi:providers:pkgA::default",
		"urn:pulumi:test::test::pkgA:m:typA::independent1",
		"urn:pulumi:test::test::pkgA:m:typA::independent2",
		"urn:pulumi:test::test::pkgA:m:typA::independent3",
		"urn:pulumi:test::test::pkgB:m:typB::failing",
		"urn:pulumi:test::test::pkgB:m:typB::failing2",
	}
	assert.Equal(t, len(expectedURNs), len(snap.Resources)) // 4 resources + 2 providers

	for _, urn := range expectedURNs {
		// Ensure that the expected URN is present in the snapshot.
		idx := slices.IndexFunc(snap.Resources, func(rs *resource.State) bool {
			return rs.URN == resource.URN(urn)
		})
		assert.NotEqual(t, -1, idx, "Expected URN %s not found in snapshot", urn)

		switch urn {
		case "urn:pulumi:test::test::pkgB:m:typB::failing", "urn:pulumi:test::test::pkgB:m:typB::failing2":
			assert.Equal(t, resource.NewStringProperty("bar"), snap.Resources[idx].Inputs["foo"])
		}
	}
}

func TestUpContinueOnErrorUpdateWithRefresh(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				UpdateF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap,
					timeout float64, ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					return nil, resource.StatusOK, errors.New("intentionally failed update")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	// update as opposed to create for the failing resource
	update := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgB:m:typB", "failing", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Inputs:                  ins,
		})
		assert.NoError(t, err)
		if update {
			assert.Equal(t, pulumirpc.Result_FAIL, resp.Result)
		} else {
			// On creation we expect to succeed
			assert.Equal(t, pulumirpc.Result_SUCCESS, resp.Result)
		}

		if update {
			respIndependent1, err := monitor.RegisterResource(
				"pkgA:m:typA", "independent1", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
				})
			assert.NoError(t, err)
			assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent1.Result)

			respIndependent2, err := monitor.RegisterResource(
				"pkgA:m:typA", "independent2", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
					Dependencies:            []resource.URN{respIndependent1.URN},
				})
			assert.NoError(t, err)
			assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent2.Result)

			respIndependent3, err := monitor.RegisterResource("pkgA:m:typA", "independent3", true, deploytest.ResourceOptions{
				SupportsResultReporting: true,
				Dependencies:            []resource.URN{respIndependent2.URN},
			})
			assert.NoError(t, err)
			assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent3.Result)
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
				Refresh:         true,
			},
			HostF: hostF,
		},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Equal(t, 2, len(snap.Resources)) // 1 resource + 1 provider

	update = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	// Run an update to create the resource
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "intentionally failed update")
	assert.NotNil(t, snap)
	assert.Equal(t, 6, len(snap.Resources)) // 4 resources + 2 providers
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgB::default"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgA::default"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent1"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent2"), snap.Resources[3].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent3"), snap.Resources[4].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgB:m:typB::failing"), snap.Resources[5].URN)
	// Refresh didn't return the input so we expect it to be empty
	assert.Equal(t, resource.PropertyValue{}, snap.Resources[5].Inputs["foo"])
}

func TestUpContinueOnErrorNoSDKSupport(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "", nil, resource.StatusOK, errors.New("intentionally failed create")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		failingResp, err := monitor.RegisterResource("pkgB:m:typB", "failing", true, deploytest.ResourceOptions{
			SupportsResultReporting: false,
		})
		assert.ErrorContains(t, err, "resource registration failed")
		assert.Nil(t, failingResp)

		respIndependent1, err := monitor.RegisterResource(
			"pkgA:m:typA", "independent1", true, deploytest.ResourceOptions{SupportsResultReporting: false})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent1.Result)

		respIndependent2, err := monitor.RegisterResource(
			"pkgA:m:typA", "independent2", true, deploytest.ResourceOptions{
				SupportsResultReporting: false,
				Dependencies:            []resource.URN{respIndependent1.URN},
			})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent2.Result)

		respIndependent3, err := monitor.RegisterResource("pkgA:m:typA", "independent3", true, deploytest.ResourceOptions{
			SupportsResultReporting: false,
			Dependencies:            []resource.URN{respIndependent2.URN},
		})
		assert.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, respIndependent3.Result)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
			},
			HostF: hostF,
		},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, "intentionally failed create")
	require.NotNil(t, snap)
	require.Equal(t, 5, len(snap.Resources)) // 3 resources + 2 providers
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgB::default"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgA::default"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent1"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent2"), snap.Resources[3].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent3"), snap.Resources[4].URN)
}

func TestUpContinueOnErrorUpdateNoSDKSupport(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				UpdateF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap,
					timeout float64, ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					return nil, resource.StatusOK, errors.New("intentionally failed update")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	// update as opposed to create for the failing resource
	update := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgB:m:typB", "failing", true, deploytest.ResourceOptions{
			SupportsResultReporting: false,
			Inputs:                  ins,
		})
		if update {
			assert.ErrorContains(t, err, "resource registration failed")
			assert.Nil(t, resp)
		} else {
			assert.NoError(t, err)
		}

		if update {
			respIndependent1, err := monitor.RegisterResource(
				"pkgA:m:typA", "independent1", true, deploytest.ResourceOptions{
					SupportsResultReporting: false,
				})
			assert.NoError(t, err)

			respIndependent2, err := monitor.RegisterResource(
				"pkgA:m:typA", "independent2", true, deploytest.ResourceOptions{
					SupportsResultReporting: false,
					Dependencies:            []resource.URN{respIndependent1.URN},
				})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "independent3", true, deploytest.ResourceOptions{
				SupportsResultReporting: false,
				Dependencies:            []resource.URN{respIndependent2.URN},
			})
			assert.NoError(t, err)
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
			},
			HostF: hostF,
		},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Equal(t, 2, len(snap.Resources)) // 1 resource + 1 provider

	update = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	// Run an update to create the resource
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "intentionally failed update")
	assert.NotNil(t, snap)
	assert.Equal(t, 6, len(snap.Resources)) // 4 resources + 2 providers
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgB::default"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgA::default"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent1"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent2"), snap.Resources[3].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::independent3"), snap.Resources[4].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgB:m:typB::failing"), snap.Resources[5].URN)
	assert.Equal(t, resource.NewStringProperty("bar"), snap.Resources[5].Inputs["foo"])
}

func TestDestroyContinueOnErrorDeleteAfterFailedUp(t *testing.T) {
	t.Parallel()
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "", nil, resource.StatusOK, errors.New("intentionally failed create")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	update := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if update {
			_, err := monitor.RegisterResource("pkgB:m:typB", "failedUp", true, deploytest.ResourceOptions{
				SupportsResultReporting: true,
			})
			assert.NoError(t, err)
		}

		if !update {
			_, err := monitor.RegisterResource("pkgA:m:typA", "willBeDeleted", true, deploytest.ResourceOptions{})
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T: t,
			// Skip display tests because different ordering makes the colouring different.
			SkipDisplayTests: true,
			UpdateOptions: UpdateOptions{
				ContinueOnError: true,
			},
			HostF: hostF,
		},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2) // We expect 1 resource + 1 provider
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgA::default"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::willBeDeleted"), snap.Resources[1].URN)

	update = true
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "intentionally failed create")
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 1) // We expect 1 provider
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgB::default"), snap.Resources[0].URN)
}
