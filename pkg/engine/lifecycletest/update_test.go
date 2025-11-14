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
	"errors"
	"strings"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
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
