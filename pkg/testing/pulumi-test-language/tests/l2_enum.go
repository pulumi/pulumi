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
	LanguageTests["l2-enum"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.EnumProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)

				require.Len(l, snapshot.Resources, 5, "3 resources & provider & stack")

				sink1 := RequireSingleNamedResource(l, snapshot.Resources, "sink1")
				sink2 := RequireSingleNamedResource(l, snapshot.Resources, "sink2")
				sink3 := RequireSingleNamedResource(l, snapshot.Resources, "sink3")

				expect := resource.PropertyMap{
					"intEnum":    resource.NewProperty(1.0),
					"stringEnum": resource.NewProperty("two"),
				}

				assert.Equal(l, expect, sink1.Outputs)
				assert.Equal(l, expect, sink2.Outputs)
				assert.Equal(l, expect, sink3.Outputs)
			},
		}},
	}
}
