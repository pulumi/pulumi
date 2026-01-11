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
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-version"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{Version: &semver.Version{Major: 1}} },
			func() plugin.Provider { return &providers.SimpleProvider{Version: &semver.Version{Major: 2}} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// We expect the following resources:
					//
					// 0. The stack
					//
					// 1. The default simple provider for V1
					// 2. The default simple provider for V2
					//
					// 3. A resource for V1
					// 4. A resource for V2
					require.Lenf(l, snap.Resources, 5, "expected 5 resources in snapshot")

					v1Resource := RequireSingleNamedResource(l, snap.Resources, "v1")
					v2Resource := RequireSingleNamedResource(l, snap.Resources, "v2")

					defaultProviderV1 := RequireSingleNamedResource(l, snap.Resources, "default_1_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProviderV1.Type.String(), "expected default simple provider")

					defaultProviderV1Ref, err := sdkproviders.NewReference(defaultProviderV1.URN, defaultProviderV1.ID)
					require.NoError(l, err, "expected to create default provider reference")

					defaultProviderV2 := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProviderV2.Type.String(), "expected default simple provider")

					defaultProviderV2Ref, err := sdkproviders.NewReference(defaultProviderV2.URN, defaultProviderV2.ID)
					require.NoError(l, err, "expected to create default provider reference")

					assert.Equal(l, v1Resource.Provider, defaultProviderV1Ref.String())
					assert.Equal(l, v2Resource.Provider, defaultProviderV2Ref.String())
				},
			},
		},
	}
}
