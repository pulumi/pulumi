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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-elide-index"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					require.Equal(l, 2, outputs.Len(), "expected 2 outputs")

					getOutput := func(key string) property.Value {
						got, ok := outputs.GetOk(key)
						require.True(l, ok, "expected property %s", key)
						return got
					}

					expectedStash := property.New("test")

					got := getOutput("stashInput")
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

					got = getOutput("stashOutput")
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:pulumi")
					myStash := RequireSingleNamedResource(l, snap.Resources, "myStash")

					want := property.NewMap(map[string]property.Value{
						"input":  expectedStash,
						"output": expectedStash,
					})
					assert.Equal(l, want, resource.FromResourcePropertyMap(myStash.Outputs), "expected myStash outputs to be %v", want)
					want = property.NewMap(map[string]property.Value{
						"input": expectedStash,
					})
					assert.Equal(l, want, resource.FromResourcePropertyMap(myStash.Inputs), "expected myStash inputs to be %v", want)
				},
			},
		},
	}
}
