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
	LanguageTests["l1-builtin-secret"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					//nolint:lll
					config.MustMakeKey("l1-builtin-secret", "aSecret"):   config.NewSecureValue("dGhpcyBpcyBhIHNlY3JldA=="), // "this is a secret" in base64
					config.MustMakeKey("l1-builtin-secret", "notSecret"): config.NewValue("this is plaintext"),
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 4)
					AssertPropertyMapMember(l, outputs,
						"roundtripSecret", resource.MakeSecret(resource.NewProperty("this is a secret")))
					AssertPropertyMapMember(l, outputs,
						"roundtripNotSecret", resource.NewProperty("this is plaintext"))
					AssertPropertyMapMember(l, outputs,
						"open", resource.NewProperty("this is a secret"))
					AssertPropertyMapMember(l, outputs,
						"close", resource.MakeSecret(resource.NewProperty("this is plaintext")))
				},
			},
		},
	}
}
