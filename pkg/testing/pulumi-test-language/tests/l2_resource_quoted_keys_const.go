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
	"github.com/stretchr/testify/require"
)

func init() {
	// Exercises two behaviors that real-provider schemas (Kubernetes) used to cover:
	//
	//   - Binding a program that writes nested typed properties with quoted keys
	//     (pulumi/pulumi#15001).
	//   - Reading a property whose schema type has a constant value.
	LanguageTests["l2-resource-quoted-keys-const"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ManifestProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					r := RequireSingleResource(l, res.Snap.Resources, "manifest:index:Resource")

					require.Equal(l, resource.PropertyMap{
						"kind": resource.NewProperty("Manifest"),
						"metadata": resource.NewProperty(resource.PropertyMap{
							"name": resource.NewProperty("first"),
							"labels": resource.NewProperty(resource.PropertyMap{
								"app": resource.NewProperty("first"),
							}),
						}),
						"spec": resource.NewProperty(resource.PropertyMap{
							"replicas": resource.NewProperty(1.0),
							"template": resource.NewProperty(resource.PropertyMap{
								"metadata": resource.NewProperty(resource.PropertyMap{
									"name": resource.NewProperty("inner"),
								}),
								"containers": resource.NewProperty([]resource.PropertyValue{
									resource.NewProperty(resource.PropertyMap{
										"name":  resource.NewProperty("app"),
										"image": resource.NewProperty("nginx"),
										"ports": resource.NewProperty([]resource.PropertyValue{
											resource.NewProperty(80.0),
										}),
									}),
								}),
							}),
						}),
					}, r.Inputs)

					AssertPropertyMapMember(l, stack.Outputs, "kind", resource.NewProperty("Manifest"))
				},
			},
		},
	}
}
