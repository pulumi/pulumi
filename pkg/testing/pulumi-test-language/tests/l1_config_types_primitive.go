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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-config-types-primitive"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-config-types-primitive", "aNumber"): config.NewValue("3.5"),
					config.MustMakeKey("l1-config-types-primitive", "aString"): config.NewValue("Hello"),
					config.MustMakeKey("l1-config-types-primitive", "aBool"):   config.NewValue("false"),
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 3, "expected 3 outputs")
					AssertPropertyMapMember(l, outputs, "theNumber", resource.NewProperty(4.75))
					AssertPropertyMapMember(l, outputs, "theString", resource.NewProperty("Hello World"))
					AssertPropertyMapMember(l, outputs, "theBool", resource.NewProperty(true))
				},
			},
		},
	}
}
