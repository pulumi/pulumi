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
	LanguageTests["l2-external-enum"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.EnumProvider{} },
			func() plugin.Provider { return &providers.ExternalEnumRefProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				RequireStackResource(l, res.Err, res.Changes)
				require.Len(l, res.Snap.Resources, 5)

				myRes := RequireSingleNamedResource(l, res.Snap.Resources, "myRes")
				assert.Equal(l, resource.PropertyMap{
					"intEnum":    resource.NewProperty(1.0),
					"stringEnum": resource.NewProperty("one"),
				}, myRes.Outputs)

				mySink := RequireSingleNamedResource(l, res.Snap.Resources, "mySink")
				assert.Equal(l, resource.PropertyMap{
					"stringEnum": resource.NewProperty("two"),
				}, mySink.Outputs)
			},
		}},
	}
}
