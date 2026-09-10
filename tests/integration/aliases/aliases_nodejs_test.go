// Copyright 2020, Pulumi Corporation.
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
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

// TestNodejsAliases tests cases where a resource's name, type, or parent changes but it provides
// an `alias` pointing to the old URN to ensure the resource is preserved across the update. Each
// scenario lives in its own scenario_*.ts file within a single program so one update exercises
// them all.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsAliases(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "step1"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		EditDirs: []integration.EditDir{
			{
				Dir:             filepath.Join("nodejs", "step2"),
				Additive:        true,
				ExpectNoChanges: true,
			},
		},
	})
}
