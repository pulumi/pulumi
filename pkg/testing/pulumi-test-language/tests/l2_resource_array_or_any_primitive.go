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
	"github.com/stretchr/testify/require"
)

func init() {
	// Exercises passing an array of primitives to an input property whose schema type is a union
	// of an array and `pulumi.json#/Any`. See https://github.com/pulumi/pulumi/issues/17181 — the
	// Python SDK used to crash when flattening such unions because it could not unwrap the list
	// element type from the Union.
	LanguageTests["l2-resource-array-or-any-primitive"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ArrayOrAnyProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					assertValues := func(name string, want property.Value) {
						r := RequireSingleNamedResource(l, res.Snap.Resources, name)
						inputs := resource.FromResourcePropertyMap(r.Inputs)
						require.Equal(l, want, inputs.Get("values"), name)
					}

					assertValues("stringList", property.New([]property.Value{
						property.New("a"),
						property.New("b"),
						property.New("c"),
					}))
					assertValues("stringAny", property.New(map[string]property.Value{
						"any": property.New(true),
					}))
				},
			},
		},
	}
}
