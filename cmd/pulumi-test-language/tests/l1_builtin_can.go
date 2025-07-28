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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-builtin-can"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-builtin-can", "aMap"):     config.NewObjectValue("{\"a\": \"MOK\"}"),
					config.MustMakeKey("l1-builtin-can", "anObject"): config.NewObjectValue("{\"a\": \"OOK\", \"opt\": null}"),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 10, "expected 10 outputs")
					AssertPropertyMapMember(l, outputs, "plainTrySuccess", resource.NewBoolProperty(true))
					AssertPropertyMapMember(l, outputs, "plainTryFailure", resource.NewBoolProperty(false))

					// The output failure variants may or may not be secret, depending on the language. We allow either.
					assertPropertyMapMember := func(
						props resource.PropertyMap,
						key string,
						want resource.PropertyValue,
					) (ok bool) {
						l.Helper()

						got, ok := props[resource.PropertyKey(key)]
						if !assert.True(l, ok, "expected property %q", key) {
							return false
						}

						if got.DeepEquals(want) {
							return true
						}
						if got.DeepEquals(resource.MakeSecret(want)) {
							return true
						}

						return assert.Equal(l, want, got, "expected property %q to be %v", key, want)
					}

					AssertPropertyMapMember(l, outputs, "outputTrySuccess",
						resource.MakeSecret(resource.NewBoolProperty(true)))
					assertPropertyMapMember(outputs, "outputTryFailure",
						resource.NewBoolProperty(false))
					AssertPropertyMapMember(l, outputs, "dynamicTrySuccess",
						resource.NewBoolProperty(true))
					assertPropertyMapMember(outputs, "dynamicTryFailure",
						resource.NewBoolProperty(false))
					AssertPropertyMapMember(l, outputs, "outputDynamicTrySuccess",
						resource.MakeSecret(resource.NewBoolProperty(true)))
					assertPropertyMapMember(outputs, "outputDynamicTryFailure",
						resource.NewBoolProperty(false))
					AssertPropertyMapMember(l, outputs, "plainTryNull", resource.NewBoolProperty(true))
					assertPropertyMapMember(outputs, "outputTryNull", resource.NewBoolProperty(true))
				},
			},
		},
	}
}
