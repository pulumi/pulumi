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
	"math"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-output-number"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					projectDirectory, err, snap, changes, events, sdks := res.ProjectDirectory, res.Err, res.Snap, res.Changes, res.Events, res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, snap, changes, events, sdks
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 6, "expected 6 outputs")
					AssertPropertyMapMember(l, outputs, "zero", resource.NewProperty(0.0))
					AssertPropertyMapMember(l, outputs, "one", resource.NewProperty(1.0))
					AssertPropertyMapMember(l, outputs, "e", resource.NewProperty(2.718))
					AssertPropertyMapMember(l, outputs, "minInt32", resource.NewProperty(float64(math.MinInt32)))
					AssertPropertyMapMember(l, outputs, "max", resource.NewProperty(math.MaxFloat64))
					AssertPropertyMapMember(l, outputs, "min", resource.NewProperty(math.SmallestNonzeroFloat64))
				},
			},
		},
	}
}
