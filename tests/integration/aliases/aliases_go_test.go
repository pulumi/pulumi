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

// TestGoAliases tests cases where a resource's name, type, or parent changes but it provides
// an `alias` pointing to the old URN to ensure the resource is preserved across the update. Each
// scenario lives in its own scenario_*.go file within a single program so one update exercises
// them all.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestGoAliases(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("go", "step1"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3=../../../sdk",
		},
		Quick: true,
		EditDirs: []integration.EditDir{
			{
				Dir:             filepath.Join("go", "step2"),
				ExpectNoChanges: true,
				Additive:        true,
			},
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestRetypeRemoteComponentAndChild(t *testing.T) {
	dir := filepath.Join("go", "retype_remote_component_and_child")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join(dir, "step1"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3=../../../sdk",
		},
		Quick: true,
		LocalProviders: []integration.LocalDependency{
			{Package: "wibble", Path: filepath.Join(dir, "provider")},
		},
		EditDirs: []integration.EditDir{
			{
				Dir:             filepath.Join(dir, "step2"),
				ExpectNoChanges: true,
				Additive:        true,
			},
		},
	})
}
