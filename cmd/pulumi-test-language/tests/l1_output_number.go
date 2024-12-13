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

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-output-number"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 6, "expected 6 outputs")
					AssertPropertyMapMember(l, outputs, "zero", resource.NewNumberProperty(0))
					AssertPropertyMapMember(l, outputs, "one", resource.NewNumberProperty(1))
					AssertPropertyMapMember(l, outputs, "e", resource.NewNumberProperty(2.718))
					AssertPropertyMapMember(l, outputs, "minInt32", resource.NewNumberProperty(math.MinInt32))
					AssertPropertyMapMember(l, outputs, "max", resource.NewNumberProperty(math.MaxFloat64))
					AssertPropertyMapMember(l, outputs, "min", resource.NewNumberProperty(math.SmallestNonzeroFloat64))
				},
			},
		},
	}
}
