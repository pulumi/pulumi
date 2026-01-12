// Copyright 2025-2026, Pulumi Corporation.
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
