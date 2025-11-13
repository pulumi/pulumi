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
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
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
