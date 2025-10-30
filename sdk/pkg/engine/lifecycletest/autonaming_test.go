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
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
)

func TestAutonaming(t *testing.T) {
	t.Parallel()

	// Track the autonaming options passed to Check
	var receivedAutonaming *plugin.AutonamingOptions

	// Create a provider that will capture the autonaming options
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					// Capture the autonaming options
					receivedAutonaming = req.Autonaming
					return plugin.CheckResponse{
						Properties: req.News,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		return err
	})

	// Create an autonamer that will return specific options
	expectedAutonaming := &plugin.AutonamingOptions{
		ProposedName:    "test",
		Mode:            plugin.AutonamingModeEnforce,
		WarnIfNoSupport: false,
	}

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
			T:     t,
			UpdateOptions: UpdateOptions{
				GeneratePlan: true,
				Autonamer:    &mockAutonamer{opts: expectedAutonaming},
			},
		},
	}

	project := p.GetProject()
	target := p.GetTarget(t, nil)

	deleteBeforeReplace := false
	plan, err := lt.TestOp(Update).Plan(project, target, p.Options, p.BackendClient, nil)
	require.NoError(t, err)
	require.NotNil(t, plan)
	for _, r := range plan.ResourcePlans {
		if r.Goal == nil {
			continue
		}
		switch r.Goal.Name {
		case "resA":
			deleteBeforeReplace = *r.Goal.DeleteBeforeReplace
		}
	}
	// Check that deleteBeforeReplace was set to true in the plan
	assert.True(t, deleteBeforeReplace)

	// Verify the autonaming options were passed correctly to provider's Check
	assert.Equal(t, expectedAutonaming, receivedAutonaming)
}

type mockAutonamer struct {
	opts *plugin.AutonamingOptions
}

func (a *mockAutonamer) AutonamingForResource(urn.URN, []byte) (*plugin.AutonamingOptions, bool) {
	return a.opts, true
}
