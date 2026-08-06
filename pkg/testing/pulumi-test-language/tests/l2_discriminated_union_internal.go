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
	"github.com/stretchr/testify/require"
)

func init() {
	// Exercises a discriminated union whose discriminator property is named
	// "type__", plus a schema-secret union property. Providers that surface a
	// wire format with a reserved discriminator key (Jackson's "__type") must
	// spell it with a trailing underscore in the schema, because a leading one
	// breaks Go and Python codegen. The benign discriminator names in the other
	// union tests never exercise underscore-adjacent identifiers at all.
	LanguageTests["l2-discriminated-union-internal"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DiscriminatedUnionInternalProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)

				alpha := resource.PropertyMap{
					"type__":  resource.NewProperty("Alpha"),
					"payload": resource.NewProperty("p1"),
					"weight":  resource.NewProperty(1.0),
				}
				betaSecret := resource.PropertyMap{
					"type__":  resource.NewProperty("Beta"),
					"payload": resource.NewProperty("s1"),
					"tint":    resource.NewProperty("blue"),
				}

				example1 := RequireSingleNamedResource(l, snapshot.Resources, "example1")
				require.Equal(l, resource.PropertyMap{
					"unionOf":     resource.NewProperty(alpha),
					"secretUnion": resource.MakeSecret(resource.NewProperty(betaSecret)),
				}, example1.Outputs)

				example2 := RequireSingleNamedResource(l, snapshot.Resources, "example2")
				require.Equal(l, resource.PropertyMap{
					"unionOf": resource.NewProperty(resource.PropertyMap{
						"type__":  resource.NewProperty("Beta"),
						"payload": resource.NewProperty("p2"),
						"tint":    resource.NewProperty("red"),
					}),
				}, example2.Outputs)

				example3 := RequireSingleNamedResource(l, snapshot.Resources, "example3")
				require.Equal(l, resource.PropertyMap{
					"unionOf": resource.NewProperty(resource.PropertyMap{
						"type__":  resource.NewProperty("Gamma"),
						"payload": resource.NewProperty("p3"),
						"active":  resource.NewProperty(true),
					}),
				}, example3.Outputs)
			},
		}},
	}
}
