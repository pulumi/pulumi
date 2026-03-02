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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["provider-custom-timeouts-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{} },
		},
		LanguageProviders: []string{"conformance-component"},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

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
				},
			},
		},
	}
}
