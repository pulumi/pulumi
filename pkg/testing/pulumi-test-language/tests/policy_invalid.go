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
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["policy-invalid"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				PolicyPacks: map[string]map[string]any{
					"invalid": nil,
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					// We expect the policy pack to fail to load due to the invalid policy name, but languages vary on
					// if they write that error to stderr or via the grpc response to be picked up in the error message,
					// so here we just check that it _did_ error.
					require.Error(l, res.Err)
				},
				Assert: func(l *L, res AssertArgs) {
					require.Error(l, res.Err)
				},
			},
		},
	}
}
