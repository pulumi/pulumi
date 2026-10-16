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
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-quoted-names"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.QuotedNamesProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 4)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:quoted-names")
					first := RequireSingleNamedResource(l, res.Snap.Resources, "first res.v1/a")
					second := RequireSingleNamedResource(l, res.Snap.Resources, "second res.v1/b")

					want := resource.PropertyMap{
						"plain": resource.NewProperty("plain"),
						"@top!": resource.NewProperty(resource.PropertyMap{
							"@in.ner/x": resource.NewProperty("inner"),
						}),
						"a.b/c": resource.NewProperty("top"),
					}
					for _, r := range []*pkgresource.State{first, second} {
						assert.Equal(l, tokens.Type("quoted-names:my.module/sub:Resource.v1/thing"), r.Type)
						assert.Equal(l, want, r.Inputs)
						assert.Equal(l, want, r.Outputs)
					}
					assert.Equal(l, []resource.URN{first.URN}, second.Dependencies)

					ignoreChanges := make([]property.Glob, len(second.IgnoreChanges))
					for i, v := range second.IgnoreChanges {
						require.NoError(l, (&ignoreChanges[i]).UnmarshalText([]byte(v)))
					}
					glob := property.GlobFromSegments
					segment := property.NewSegment[string]
					assert.Equal(l, []property.Glob{
						glob(segment("a.b/c")),
						glob(segment("@top!"), segment("@in.ner/x")),
					}, ignoreChanges)

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					assert.Equal(l, resource.PropertyMap{
						"invoked":      resource.NewProperty(want),
						"invokedInner": resource.NewProperty("inner"),
					}, stack.Outputs)
				},
			},
		},
	}
}
