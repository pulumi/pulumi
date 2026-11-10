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
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-builtin-try"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-builtin-try", "aMap"):     config.NewObjectValue("{\"a\": \"MOK\"}"),
					config.MustMakeKey("l1-builtin-try", "anObject"): config.NewObjectValue("{\"a\": \"OOK\", \"opt\": null}"),
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					require.Equal(l, 10, outputs.Len(), "expected 10 outputs")
					AssertPropertyMapMember(l, outputs, "plainTrySuccess", property.New("MOK"))
					AssertPropertyMapMember(l, outputs, "plainTryFailure", property.New("fallback"))

					// The output failure variants may or may not be secret, depending on the language. We allow either.
					assertPropertyMapMember := func(
						props property.Map,
						key string,
						want property.Value,
					) (ok bool) {
						l.Helper()

						got, ok := props.GetOk(key)
						if !assert.True(l, ok, "expected property %q", key) {
							return false
						}

						if got.Equals(want) {
							return true
						}
						if got.Equals(want.WithSecret(true)) {
							return true
						}

						return assert.Equal(l, want, got, "expected property %q to be %v", key, want)
					}

					AssertPropertyMapMember(l, outputs, "outputTrySuccess",
						property.New("MOK").WithSecret(true))
					assertPropertyMapMember(outputs, "outputTryFailure",
						property.New("fallback"))
					AssertPropertyMapMember(l, outputs, "dynamicTrySuccess",
						property.New("OOK"))
					assertPropertyMapMember(outputs, "dynamicTryFailure",
						property.New("fallback"))
					AssertPropertyMapMember(l, outputs, "outputDynamicTrySuccess",
						property.New("OOK").WithSecret(true))
					assertPropertyMapMember(outputs, "outputDynamicTryFailure",
						property.New("fallback"))
					AssertPropertyMapMember(l, outputs, "plainTryNull",
						property.New([]property.Value{property.New(property.Null)}))

					// This may be secret at the list level, or the element level, or none
					got, ok := outputs.GetOk("outputTryNull")
					if assert.True(l, ok, "expected property outputTryNull") {
						// Must equal one of the following

						ok := slices.ContainsFunc([]property.Value{
							property.New([]property.Value{property.New(property.Null)}),
							property.New([]property.Value{property.New(property.Null)}).WithSecret(true),
							property.New([]property.Value{property.New(property.Null).WithSecret(true)}),
						}, func(v property.Value) bool { return got.Equals(v) })

						if !ok {
							assert.Fail(l, "expected property outputTryNull to be one of the expected values, was %v", got)
						}
					}
				},
			},
		},
	}
}
