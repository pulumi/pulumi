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

package lifecycletest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
)

func TestComponentResourceTypeAliasWithReadResource(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	var componentURN resource.URN
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(
			"pkg:index:typA.Repro",
			"test",
			false,
			deploytest.ResourceOptions{},
		)
		require.NoError(t, err)
		componentURN = resp.URN

		_, _, err = monitor.ReadResource(
			"pkgA:iam:Policy",
			"AWSBackupServiceRolePolicyForBackup",
			"arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup",
			componentURN, // parent
			resource.PropertyMap{},
			"",
			"",
			"",
			nil,
			"",
			"",
		)
		require.NoError(t, err)

		err = monitor.RegisterResourceOutputs(componentURN, resource.PropertyMap{})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 3)
	require.Equal(t, urn.URN("urn:pulumi:test::test::pkg:index:typA.Repro::test"), snap.Resources[0].URN)
	require.Equal(t, urn.URN("urn:pulumi:test::test::pkg:index:typA.Repro::test"), snap.Resources[2].Parent)

	programF2 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(
			"pkg:typA:Repro",
			"test",
			false,
			deploytest.ResourceOptions{
				Aliases: []*pulumirpc.Alias{
					makeSpecAlias("test", "pkg:index:typA.Repro", "", ""),
				},
			},
		)
		require.NoError(t, err)
		componentURN = resp.URN

		_, _, err = monitor.ReadResource(
			"pkgA:iam:Policy",
			"AWSBackupServiceRolePolicyForBackup",
			"arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup",
			componentURN, // parent
			resource.PropertyMap{},
			"",
			"",
			"",
			nil,
			"",
			"",
		)
		require.NoError(t, err)

		err = monitor.RegisterResourceOutputs(componentURN, resource.PropertyMap{})
		require.NoError(t, err)

		return nil
	})

	hostF2 := deploytest.NewPluginHostF(nil, nil, programF2, loaders...)
	p2 := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF2},
	}

	project2 := p2.GetProject()
	snap2, err := lt.TestOp(Update).RunStep(project2, p2.GetTarget(t, snap), p2.Options, false, p2.BackendClient, nil, "1")

	require.NoError(t, err)
	require.Len(t, snap2.Resources, 3, "Expected no duplicate resources after alias update")

	require.Equal(t, urn.URN("urn:pulumi:test::test::pkg:typA:Repro::test"), snap2.Resources[0].URN)
	require.Equal(t, urn.URN("urn:pulumi:test::test::pkg:typA:Repro::test"), snap2.Resources[2].Parent)
}

func TestUpdateWithTargetedParentChildMarkedAsDelete(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	snap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		resA := &resource.State{
			Type: "pkgA:m:typA",
			URN:  p.NewURN("pkgA:m:typA", "resA", ""),
		}
		s.Resources = append(s.Resources, resA)

		justAChild := &resource.State{
			Type:   "pkgA:m:typA",
			URN:    "urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::childA",
			Custom: true,
			Delete: true,
			Parent: resA.URN,
			ID:     "id1",
		}
		s.Resources = append(s.Resources, justAChild)

		return s
	}()
	require.NoError(t, snap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	host := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: host,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				string(p.NewURN("pkgA:m:typA", "resA", "")),
			}),
		},
	}

	validationFunc := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		events []Event, err error,
	) error {
		foundError := false
		for _, e := range events {
			if e.Type == DiagEvent {
				payload := e.Payload().(DiagEventPayload)
				//nolint:lll // The error message is long
				if strings.Contains(
					payload.Message,
					"Resource 'urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::childA' will be destroyed but was not specified in --target list.") {
					foundError = true
				}
				opts.T.Logf("%s: %s", payload.Severity, payload.Message)
			}
		}
		if !foundError {
			return errors.New("expected error not found")
		}

		return err
	}

	_, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, validationFunc, "1")
	require.ErrorContains(t, err, "step generator errored")
}

func TestUpdateWithTargetedResourceChangingParent(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#21404]: Fix the underlying issue and re-enable this test.
	t.Skip("Skipping test due to underlying panic")

	p := &lt.TestPlan{}

	initialSnap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		prov := &resource.State{
			Type:   "pulumi:providers:pkgA",
			URN:    p.NewProviderURN("pkgA", "prov", ""),
			Custom: true,
			ID:     "id",
		}
		s.Resources = append(s.Resources, prov)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		parent := &resource.State{
			Type:     "pkgA:m:typA",
			URN:      p.NewURN("pkgA:m:typA", "parent", ""),
			Provider: provRef.String(),
		}
		s.Resources = append(s.Resources, parent)

		resA := &resource.State{
			Type:               "pkgA:m:typA",
			URN:                p.NewURN("pkgA:m:typA", "resA", parent.URN),
			Custom:             true,
			ID:                 "id",
			PendingReplacement: true,
			Provider:           provRef.String(),
			Parent:             parent.URN,
		}
		s.Resources = append(s.Resources, resA)

		return s
	}()
	require.NoError(t, initialSnap.VerifyIntegrity())

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		parentURN := p.NewURN("pkgA:m:typA", "parent", "")
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
			AliasURNs: []resource.URN{
				p.NewURN("pkgA:m:typA", "resA", parentURN),
			},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: engine.UpdateOptions{
			Refresh:        true,
			RefreshProgram: true,
		},
	}

	_, err := lt.TestOp(engine.Update).
		RunStep(p.GetProject(), p.GetTarget(t, initialSnap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

func TestTargetedUpdateWithProviderDependencyOnAliasedResource(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#21426]: Fix the underlying issue and re-enable this test.
	t.Skip("Skipping test due to underlying snapshot integrity issue")

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	setupSnap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		provA := &resource.State{
			Type:   "pulumi:providers:pkgA",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkgA::prov",
			Custom: true,
			ID:     "id-prov",
		}
		s.Resources = append(s.Resources, provA)

		provRef, err := providers.NewReference(provA.URN, provA.ID)
		require.NoError(t, err)

		parent := &resource.State{
			Type:     "pkgA:index:Component",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:index:Component::parent",
			Custom:   false,
			Provider: provRef.String(),
		}
		s.Resources = append(s.Resources, parent)

		child := &resource.State{
			Type:     "pkgA:index:Res",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:index:Component$pkgA:index:Res::child",
			Custom:   true,
			ID:       "id-child",
			Provider: provRef.String(),
			Parent:   parent.URN,
		}
		s.Resources = append(s.Resources, child)

		provB := &resource.State{
			Type:   "pulumi:providers:pkgB",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkgB::provB",
			Custom: true,
			ID:     "id-provB",
			Dependencies: []resource.URN{
				child.URN,
			},
		}
		s.Resources = append(s.Resources, provB)

		return s
	}()
	require.NoError(t, setupSnap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		parent, err := monitor.RegisterResource("pkgA:index:Component", "parent", false, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:Res", "child", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
			Parent:   parent.URN,
			AliasURNs: []resource.URN{
				"urn:pulumi:test-stack::test-project::pkgA:index:Res::child",
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkgB", "provB", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{
				"urn:pulumi:test-stack::test-project::pkgA:index:Res::child",
			},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	opts := lt.TestUpdateOptions{
		T:             t,
		HostF:         hostF,
		UpdateOptions: engine.UpdateOptions{},
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, setupSnap), opts, false, p.BackendClient, nil, "1")
	require.Error(t, err) // TODO: Change this to require.ErrorContains with the correct error message
}

func TestUpdateDeletedWithResourceDependedsOnDeleteResource(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#21433]: Fix the underlying issue and re-enable this test.
	t.Skip("Skipping test due to underlying snapshot integrity issue")
	// Note: this test might be testing an invalid scenario, and maybe we should try to prevent
	// that scenario in the first place. It might not be possible to get into this scenario except via state edits
	// though, you'd need a replace operation which failed to delete the old resource, and then you'd have to
	// manually state edit to remove the newly replaced copy of the resource. Maybe we should just make that
	// initial state an SIE? Check that all dependencies are pointing to existing resources, so
	// you wouldn't even be able to import a state like that.

	p := &lt.TestPlan{}
	project := p.GetProject()

	snap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		provA := &resource.State{
			Type:   "pulumi:providers:pkgA",
			URN:    p.NewProviderURN("pkgA", "provA", ""),
			Custom: true,
			ID:     "id1",
		}
		s.Resources = append(s.Resources, provA)

		provRefA, err := providers.NewReference(provA.URN, provA.ID)
		require.NoError(t, err)

		resA := &resource.State{
			Type:     "pkgA:m:typA",
			URN:      p.NewURN("pkgA:m:typA", "resA", ""),
			Custom:   false,
			Delete:   true,
			Provider: provRefA.String(),
		}
		s.Resources = append(s.Resources, resA)

		provB := &resource.State{
			Type:        "pulumi:providers:pkgB",
			URN:         p.NewProviderURN("pkgB", "provB", ""),
			Custom:      true,
			ID:          "id2",
			DeletedWith: resA.URN,
		}
		s.Resources = append(s.Resources, provB)

		return s
	}()
	require.NoError(t, snap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkgB", "provB", true, deploytest.ResourceOptions{
			DeletedWith: p.NewURN("pkgA:m:typA", "resA", ""),
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

func TestPendingReplacementUpdateSnapshotIntegrity(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#21700]: Fix the underlying issue and re-enable this test. Note that this test is flaky,
	// so you need to run it with `-count=1000` to make sure you see the failure.
	t.Skip("Skipping test due to underlying panic")

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	setupSnap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		provA := &resource.State{
			Type:   "pulumi:providers:pkgA",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkgA::provA",
			Custom: true,
			ID:     "id1",
		}
		s.Resources = append(s.Resources, provA)

		provRef, err := providers.NewReference(provA.URN, provA.ID)
		require.NoError(t, err)

		resA := &resource.State{
			Type:               "pkgA:m:typA",
			URN:                "urn:pulumi:test-stack::test-project::pkgA:m:typA::resA",
			Custom:             true,
			ID:                 "id2",
			PendingReplacement: true,
			Provider:           provRef.String(),
		}
		s.Resources = append(s.Resources, resA)

		return s
	}()
	require.NoError(t, setupSnap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: engine.UpdateOptions{
			Refresh:        true,
			RefreshProgram: true,
		},
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, setupSnap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

func TestUntargetedComponentResource(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)

					// This call to RegisterResourceOutputs triggers the panic when the component
					// is filtered out by targeting. The panic occurs because findResourceInNewOrOld()
					// cannot locate the resource in either the base snapshot or the new resources map.
					outs := resource.PropertyMap{}
					err = monitor.RegisterResourceOutputs(resp.URN, outs)
					require.NoError(t, err)

					return plugin.ConstructResponse{
						URN:     resp.URN,
						Outputs: outs,
					}, nil
				},
			}, nil
		}),
	}

	registerComponent := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "existingResource", true)
		require.NoError(t, err)

		if registerComponent {
			_, _ = monitor.RegisterResource("pkgA:m:typA", "newComponent", false, deploytest.ResourceOptions{
				Remote: true,
			})
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	opts := lt.TestUpdateOptions{T: t, HostF: hostF}
	snap, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	existingResourceURN := p.NewURN("pkgA:m:typA", "existingResource", "")
	opts.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{existingResourceURN})
	registerComponent = true

	snap, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	for _, res := range snap.Resources {
		require.NotContains(t, res.URN, "newComponent",
			"newComponent should not be in the snapshot because it's not being targeted")
	}
}

// TestTargetedUpdateRefreshUnknownChildProvider reproduces a fuzz-found snapshot integrity error
// where a targeted update with refresh produces an unknown provider reference for child providers.
func TestTargetedUpdateRefreshUnknownChildProvider(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#22511]: Fix the underlying issue and re-enable this test.
	t.Skip("Skipping: targeted update with refresh produces unknown provider reference for child providers")

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	// Set up the initial snapshot.
	setupSnap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		prov0 := &resource.State{
			Type:   "pulumi:providers:pkg-mt06",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-mt06::res-b0L9",
			Custom: true,
			ID:     "id-aQ2mk2A3Gc2W",
		}
		s.Resources = append(s.Resources, prov0)

		provRef0, err := providers.NewReference(prov0.URN, prov0.ID)
		require.NoError(t, err)

		res1 := &resource.State{
			Type:           "pkg-mt06:mod-qaO0:type-qit2",
			URN:            "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2::res-h66B",
			Custom:         false,
			ID:             "",
			Protect:        true,
			RetainOnDelete: true,
			Provider:       provRef0.String(),
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res1)

		prov2 := &resource.State{
			Type:   "pulumi:providers:pkg-aL11",
			URN:    "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pulumi:providers:pkg-aL11::res-daE8",
			Custom: true,
			ID:     "id-d5mE0e3du8zy",
			Parent: res1.URN,
		}
		s.Resources = append(s.Resources, prov2)

		provRef2, err := providers.NewReference(prov2.URN, prov2.ID)
		require.NoError(t, err)

		res3URN := "urn:pulumi:test-stack::test-project::" +
			"pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-dXI1:type-az15::res-q03W"
		res3 := &resource.State{
			Type:           "pkg-aL11:mod-dXI1:type-az15",
			URN:            resource.URN(res3URN),
			Custom:         false,
			ID:             "",
			Protect:        true,
			RetainOnDelete: true,
			Provider:       provRef2.String(),
			Parent:         res1.URN,
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res3)

		res4 := &resource.State{
			Type:           "pkg-aL11:mod-dXI1:type-az15",
			URN:            "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W",
			Custom:         false,
			Delete:         true,
			ID:             "",
			RetainOnDelete: true,
			Provider:       provRef2.String(),
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res4)

		res5URN := "urn:pulumi:test-stack::test-project::" +
			"pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1"
		res5 := &resource.State{
			Type:               "pkg-aL11:mod-gD82:type-x615",
			URN:                resource.URN(res5URN),
			Custom:             false,
			ID:                 "",
			Protect:            true,
			PendingReplacement: true,
			Provider:           provRef2.String(),
			Parent:             res1.URN,
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res5)

		return s
	}()
	require.NoError(t, setupSnap.VerifyIntegrity(), "initial snapshot is not valid")

	// Set up the reproduction providers and program.
	createF := func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-f2JG::res-cz8e":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-z1M1:type-m01D::res-f1FH":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)
		}
		return plugin.CreateResponse{
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}
	deleteF := func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a0g6:type-joBK::res-a4hs":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-aL11::res-daE8":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-k7ME:type-lWJ0::res-a7eK":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-zRIm:type-v1Jd::res-oPfG":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-o903:type-fC14::res-u0Ji":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2::res-h66B":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)
		}
		return plugin.DeleteResponse{
			Status: resource.StatusOK,
		}, nil
	}
	diffF := func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2::res-h66B":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: true,
			}, nil

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1":
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-f2JG::res-cz8e":
			return plugin.DiffResponse{}, fmt.Errorf("diff failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a0g6:type-joBK::res-a4hs":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: true,
			}, nil

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-mt06::res-b0L9":
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: false,
			}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-k7ME:type-lWJ0::res-a7eK":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: true,
			}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
		}
		return plugin.DiffResponse{}, nil
	}
	readF := func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-o903:type-fC14::res-u0Ji":
			return plugin.ReadResponse{}, nil

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-f2JG::res-cz8e":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-aL11::res-daE8":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.ReadResponse{}, nil

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-z1M1:type-m01D::res-f1FH":
			return plugin.ReadResponse{}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-zRIm:type-v1Jd::res-oPfG":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)
		}
		return plugin.ReadResponse{
			ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},
			Status:     resource.StatusOK,
		}, nil
	}
	updateF := func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-z1M1:type-m01D::res-f1FH":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a0g6:type-joBK::res-a4hs":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)
		}
		return plugin.UpdateResponse{
			Properties: req.NewInputs,
			Status:     resource.StatusOK,
		}, nil
	}

	reproLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkg-aL11", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: createF,
				DeleteF: deleteF,
				DiffF:   diffF,
				ReadF:   readF,
				UpdateF: updateF,
			}, nil
		}),
		deploytest.NewProviderLoader("pkg-f2JG", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: createF,
				DeleteF: deleteF,
				DiffF:   diffF,
				ReadF:   readF,
				UpdateF: updateF,
			}, nil
		}),
		deploytest.NewProviderLoader("pkg-mt06", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: createF,
				DeleteF: deleteF,
				DiffF:   diffF,
				ReadF:   readF,
				UpdateF: updateF,
			}, nil
		}),
	}

	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov0, err := monitor.RegisterResource("pulumi:providers:pkg-f2JG", "res-cz8e", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		provRef0, err := providers.NewReference(prov0.URN, prov0.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-a0g6:type-joBK", "res-a4hs", true, deploytest.ResourceOptions{
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkg-f2JG:mod-z1M1:type-m01D", "res-f1FH", true, deploytest.ResourceOptions{
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkg-mt06", "res-b0L9", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res4, err := monitor.RegisterResource("pkg-f2JG:mod-a6Jt:type-fi4L", "res-aE61", false, deploytest.ResourceOptions{
			Protect:        ptr(true),
			RetainOnDelete: ptr(true),
			Provider:       provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkg-aL11", "res-daE8", true, deploytest.ResourceOptions{
			Protect:        ptr(true),
			RetainOnDelete: ptr(true),
		})
		require.NoError(t, err)

		res6, err := monitor.RegisterResource("pkg-aL11:mod-dXI1:type-az15", "res-q03W", false, deploytest.ResourceOptions{
			Protect: ptr(true),
			AliasURNs: []resource.URN{
				"urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-dXI1:type-az15::res-q03W",
			},
		})
		require.NoError(t, err)

		res7, err := monitor.RegisterResource("pkg-f2JG:mod-zRIm:type-v1Jd", "res-oPfG", true, deploytest.ResourceOptions{
			RetainOnDelete: ptr(true),
			Provider:       provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-k7ME:type-lWJ0", "res-a7eK", false, deploytest.ResourceOptions{
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-aDm1:type-hu4N", "res-rB12", true, deploytest.ResourceOptions{
			Provider: provRef0.String(),
			Dependencies: []resource.URN{
				res4.URN,
				res6.URN,
				res7.URN,
			},
			DeletedWith: res2.URN,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-o903:type-fC14", "res-u0Ji", false, deploytest.ResourceOptions{
			Protect:  ptr(true),
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		return nil
	})

	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, reproLoaders...)
	reproOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
		UpdateOptions: engine.UpdateOptions{
			Refresh: true,
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1",
			}),
		},
	}

	// Trigger the reproduction.
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
