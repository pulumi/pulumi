// Copyright 2026, Pulumi Corporation.
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestPackageRefSharedBase is a regression test for https://github.com/pulumi/pulumi/issues/24336. Check that
// we don't return a parmameterized reference for a non-parameterized package of the same base provider.
func TestPackageRefSharedBase(t *testing.T) {
	t.Parallel()

	for _, asExtension := range []bool{false, true} {
		title := "replacement"
		if asExtension {
			title = "extension"
		}
		t.Run(title, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{}, nil
				}),
			}

			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				parameterization := &pulumirpc.Parameterization{
					Name:    "extPkg",
					Version: "2.0.0",
					Value:   []byte{0, 1, 2, 3},
				}

				var replacement, extension *pulumirpc.Parameterization
				if asExtension {
					extension = parameterization
				} else {
					replacement = parameterization
				}

				pkg1Ref, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, replacement, extension)
				require.NoError(t, err)

				pkg2Ref, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil, nil)
				require.NoError(t, err)

				// These two packages should have resolved to different references, since one is parameterized and the other is not.
				assert.NotEqual(t, pkg1Ref, pkg2Ref)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}

			snap, err := lt.TestOp(Update).
				RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
			require.NoError(t, err)
			require.NotNil(t, snap)
		})
	}
}
