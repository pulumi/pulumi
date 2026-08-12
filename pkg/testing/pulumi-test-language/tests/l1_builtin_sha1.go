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
	"crypto/sha1" //nolint:gosec // we don't need a strong cryptographic primitive
	"encoding/hex"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	addTest := func(input string) TestRun {
		h := sha1.Sum([]byte(input)) //nolint:gosec // we don't need a strong cryptographic primitive
		expected := hex.EncodeToString(h[:])

		return TestRun{
			Config: config.Map{
				config.MustMakeKey("l1-builtin-sha1", "input"): config.NewValue(input),
			},
			Assert: func(l *L, res AssertArgs) {
				require.NoError(l, res.Err)
				stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
				assert.Equal(l, resource.PropertyMap{
					"hash": resource.NewProperty(expected),
				}, stack.Outputs)
			},
		}
	}

	LanguageTests["l1-builtin-sha1"] = LanguageTest{
		RunsShareSource: true,
		Runs: []TestRun{
			addTest("hello world"),
			addTest("goodbye world"),
		},
	}
}
