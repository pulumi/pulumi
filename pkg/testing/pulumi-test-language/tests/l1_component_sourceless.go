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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-component-sourceless"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					snap := res.Snap
					require.Len(l, snap.Resources, 2, "expected 2 resources in snapshot")

					// A component declared without a source registers under the type token it names, rather
					// than under one derived from its declaration.
					component := RequireSingleResource(l, snap.Resources, "my:custom:Component")
					assert.Equal(l, "myComponent", component.URN.Name())

					want := resource.NewPropertyMapFromMap(map[string]any{
						"aNumber": 42,
						"aString": "hello",
					})
					assert.Equal(l, want, component.Inputs, "expected component inputs to be %v", want)
				},
			},
		},
	}
}
