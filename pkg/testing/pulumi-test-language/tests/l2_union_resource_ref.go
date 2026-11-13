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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l2-union-resource-ref"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.UnionRefProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				RequireStackResource(l, res.Err, res.Changes)
				RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:unionref")

				ref := func(name string) property.Value {
					r := RequireSingleNamedResource(l, res.Snap.Resources, name)
					return resource.FromResourcePropertyValue(resource.MakeCustomResourceReference(r.URN, r.ID, ""))
				}
				github, gitlab := ref("github"), ref("gitlab")

				pipeline := RequireSingleNamedResource(l, res.Snap.Resources, "pipeline")
				assert.Equal(l, property.NewMap(map[string]property.Value{
					"source":  github,
					"program": property.New(map[string]property.Value{"source": gitlab}),
					"sources": property.New([]property.Value{github, gitlab}),
				}), resource.FromResourcePropertyMap(pipeline.Inputs))
			},
		}},
	}
}
