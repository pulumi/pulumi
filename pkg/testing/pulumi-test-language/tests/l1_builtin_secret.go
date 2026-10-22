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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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

					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					require.Equal(l, 5, outputs.Len())
					AssertPropertyMapMember(l, outputs,
						"roundtripSecret", property.New("this is a secret").WithSecret(true))
					AssertPropertyMapMember(l, outputs,
						"roundtripNotSecret", property.New("this is plaintext"))
					AssertPropertyMapMember(l, outputs,
						"open", property.New("this is a secret"))
					AssertPropertyMapMember(l, outputs,
						"close", property.New("this is plaintext").WithSecret(true))
					AssertPropertyMapMember(l, outputs,
						"double", property.New("this is a secret").WithSecret(true))
				},
			},
		},
	}
}
