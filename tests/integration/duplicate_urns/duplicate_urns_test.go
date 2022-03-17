// Copyright 2016-2018, Pulumi Corporation.
// +build nodejs all
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

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

// Test that the engine does not tolerate duplicate URNs in the same plan.
func TestDuplicateURNs(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:           "step1",
		Dependencies:  []string{"@pulumi/pulumi"},
		Quick:         true,
		ExpectFailure: true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "step2",
				Additive: true,
			},
			{
				Dir:           "step3",
				Additive:      true,
				ExpectFailure: true,
			},
			{
				Dir:           "step4",
				Additive:      true,
				ExpectFailure: true,
			},
		},
	})
}
