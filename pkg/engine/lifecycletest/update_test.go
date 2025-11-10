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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func TestChildMarkedAsDelete(t *testing.T) {
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
		UpdateOptions: engine.UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				string(p.NewURN("pkgA:m:typA", "resA", "")),
			}),
		},
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
