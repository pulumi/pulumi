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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-builtin-check-pulumi-version"] = LanguageTest{
		Runs: []TestRun{
			{
				// A version range that is not satisfied by the CLI version used in the tests
				Config: config.Map{
					config.MustMakeKey("l1-builtin-check-pulumi-version", "version"): config.NewValue("=3.1.2"),
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result on preview")
					// Ideally we would capture the error logged to stdout/err by the language runtime here, but we
					// don't have a way to do that.
					// `Pulumi CLI version .* does not satisfy the version range \"=3\\.1\\.2\"`
				},
				Assert: func(l *L, res AssertArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result on up")
					// Ideally we would capture the error logged to stdout/err by the language runtime here, but we
					// don't have a way to do that.
					// `Pulumi CLI version .* does not satisfy the version range \"=3\\.1\\.2\"`
				},
			},
			{
				// A version range that is satisfied by the CLI version used in the tests
				Config: config.Map{
					config.MustMakeKey("l1-builtin-check-pulumi-version", "version"): config.NewValue(">=3.0.0"),
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.Nil(l, res.Err)
				},
				Assert: func(l *L, res AssertArgs) {
					require.Nil(l, res.Err)
				},
			},
		},
	}
}
