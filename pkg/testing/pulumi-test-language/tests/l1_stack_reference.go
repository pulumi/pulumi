// Copyright 2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-stack-reference"] = LanguageTest{
		StackReferences: map[string]property.Map{
			"organization/other/dev": property.NewMap(map[string]property.Value{
				"plain":  property.New("plain"),
				"secret": property.New("secret").WithSecret(true),
			}),
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					require.Len(l, snap.Resources, 3, "expected at least 3 resources")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:pulumi")
					RequireSingleResource(l, snap.Resources, "pulumi:pulumi:StackReference")

					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					require.Equal(l, 3, outputs.Len(), "expected 3 outputs")
					AssertPropertyMapMember(l, outputs, "plain", property.New("plain"))
					AssertPropertyMapMember(l, outputs, "secret", property.New("secret").WithSecret(true))
					AssertPropertyMapMember(l, outputs, "secret_unsecret", property.New("secret"))
				},
			},
		},
	}
}
