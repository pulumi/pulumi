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

package tests

import (
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	createRun := func(timeout string, seconds float64) TestRun {
		return TestRun{
			Config: config.Map{
				config.MustMakeKey("l2-resource-option-custom-timeouts", "createTimeout"): config.NewValue(timeout),
			},
			Assert: func(l *L, res AssertArgs) {
				require.NoError(l, res.Err)
				require.Len(l, res.Snap.Resources, 8)

				noTimeouts := RequireSingleNamedResource(l, res.Snap.Resources, "noTimeouts")
				createOnly := RequireSingleNamedResource(l, res.Snap.Resources, "createOnly")
				updateOnly := RequireSingleNamedResource(l, res.Snap.Resources, "updateOnly")
				deleteOnly := RequireSingleNamedResource(l, res.Snap.Resources, "deleteOnly")
				allTimeouts := RequireSingleNamedResource(l, res.Snap.Resources, "allTimeouts")

				assert.Equal(l, resource.CustomTimeouts{}, noTimeouts.CustomTimeouts)

				require.Equal(l, resource.CustomTimeouts{
					Create: 300,
				}, createOnly.CustomTimeouts)

				assert.Equal(l, resource.CustomTimeouts{
					Update: 600,
				}, updateOnly.CustomTimeouts)

				assert.Equal(l, resource.CustomTimeouts{
					Delete: 180,
				}, deleteOnly.CustomTimeouts)

				assert.Equal(l, resource.CustomTimeouts{
					Create: 120,
					Update: 240,
					Delete: 60,
				}, allTimeouts.CustomTimeouts)

				configTimeout := RequireSingleNamedResource(l, res.Snap.Resources, "configTimeout")
				assert.Equal(l, resource.CustomTimeouts{
					Create: seconds,
				}, configTimeout.CustomTimeouts)
			},
		}
	}

	LanguageTests["l2-resource-option-custom-timeouts"] = LanguageTest{
		RunsShareSource: true,
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			createRun("7m", 420),
			createRun("1h", 60*60),
			createRun(".2m", 12),
		},
	}
}
