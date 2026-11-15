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
	LanguageTests["l2-resource-ref-in-object-type"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ObjectRefProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				RequireStackResource(l, res.Err, res.Changes)
				RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:objectref")

				for _, c := range []struct{ referenced, referencing, refProp, prop string }{
					{"widget", "gadget", "widgetRef", "widget"},
					{"alpha", "beta", "alphaRef", "alpha"},
				} {
					referenced := RequireSingleNamedResource(l, res.Snap.Resources, c.referenced)
					referencing := RequireSingleNamedResource(l, res.Snap.Resources, c.referencing)
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"settings": property.New(map[string]property.Value{"name": property.New(c.referenced)}),
					}), resource.FromResourcePropertyMap(referenced.Inputs))

					ref := resource.FromResourcePropertyValue(
						resource.MakeCustomResourceReference(referenced.URN, referenced.ID, ""))
					assert.Equal(l, property.NewMap(map[string]property.Value{
						c.refProp: property.New(map[string]property.Value{c.prop: ref}),
					}), resource.FromResourcePropertyMap(referencing.Inputs))
				}
			},
		}},
	}
}
