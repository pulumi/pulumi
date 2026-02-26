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
	"encoding/base64"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	addTest := func(input string) TestRun {
		bytes, err := base64.StdEncoding.DecodeString(input)
		contract.AssertNoErrorf(err, "expected valid base64 string")

		return TestRun{
			Config: config.Map{
				config.MustMakeKey("l1-builtin-base64", "input"): config.NewValue(input),
			},
			Assert: func(l *L, res AssertArgs) {
				require.NoError(l, res.Err)
				stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
				want := resource.PropertyMap{
					"data":      resource.NewProperty(string(bytes)),
					"roundtrip": resource.NewProperty(input),
				}

				assert.Equal(l, want, stack.Outputs, "expected stack outputs to be %v", want)
			},
		}
	}

	LanguageTests["l1-builtin-base64"] = LanguageTest{
		RunsShareSource: true,
		Runs: []TestRun{
			addTest("aGVsbG8gd29ybGQ="),
			addTest("Z29vZGJ5ZSB3b3JsZA=="),
		},
	}
}
