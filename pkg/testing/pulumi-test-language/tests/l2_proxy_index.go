// Copyright 2025, Pulumi Corporation.
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
	LanguageTests["l2-proxy-index"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.RefRefProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					require.Equal(l, 5, outputs.Len(), "expected 5 outputs")
					AssertPropertyMapMember(l, outputs, "bool", property.New(true))
					AssertPropertyMapMember(l, outputs, "array", property.New(true))
					AssertPropertyMapMember(l, outputs, "map", property.New("100"))
					AssertPropertyMapMember(l, outputs, "nested", property.New("french hens"))
					AssertPropertyMapMember(l, outputs, "listIndex", property.New("Partridge"))
				},
			},
		},
	}
}
