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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-ignore-changes"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// The stack, default provider, and 3 explicit resources:
					// receiverIgnore, mapIgnore, and noIgnore.
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					receiverIgnore := RequireSingleNamedResource(l, snap.Resources, "receiverIgnore")
					assert.Equal(l, []string{"details[0].key"}, receiverIgnore.IgnoreChanges)

					mapIgnore := RequireSingleNamedResource(l, snap.Resources, "mapIgnore")
					assert.Equal(l, []string{
						`tags["env"]`,
						`tags["with.dot"]`,
						`tags["with escaped \""]`,
					}, mapIgnore.IgnoreChanges)

					noIgnore := RequireSingleNamedResource(l, snap.Resources, "noIgnore")
					assert.Empty(l, noIgnore.IgnoreChanges)
				},
			},
		},
	}
}
