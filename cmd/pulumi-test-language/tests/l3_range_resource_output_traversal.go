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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-range-resource-output-traversal"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DNSProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Expect: stack, dns provider, subscription, 2 records (one per domain), and 1 record provider.
					// Actually the range creates one record per challenge entry.
					// We have 2 domains -> 2 challenges -> 2 records.
					// Resources: stack + dns provider + subscription + 2 records = 5
					require.Len(l, res.Snap.Resources, 5, "expected 5 resources in snapshot")

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:dns")
					RequireSingleResource(l, res.Snap.Resources, "dns:index:Subscription")

					records := RequireNResources(l, res.Snap.Resources, "dns:index:Record", 2)
					for _, record := range records {
						name, ok := record.Inputs["name"]
						assert.True(l, ok, "expected record to have 'name' input")
						assert.True(l, name.IsString(), "expected 'name' to be a string")
						assert.Contains(l, name.StringValue(), "_acme-challenge.",
							"expected name to contain '_acme-challenge.'")
					}
				},
			},
		},
	}
}
