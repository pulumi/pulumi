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
	"strings"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-large-string"] = LanguageTest{
		Providers: []plugin.Provider{&providers.LargeProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					// Check that the large string is in the snapshot
					largeString := resource.NewStringProperty(strings.Repeat("hello world", 9532509))
					large := RequireSingleResource(l, snap.Resources, "large:index:String")
					require.Equal(l,
						resource.NewStringProperty("hello world"),
						large.Inputs["value"],
					)
					require.Equal(l,
						largeString,
						large.Outputs["value"],
					)

					// Check the stack output value is as well
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					require.Equal(l, largeString, stack.Outputs["output"], "expected large string stack output")
				},
			},
		},
	}
}
