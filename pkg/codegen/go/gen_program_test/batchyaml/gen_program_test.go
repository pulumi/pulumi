// Copyright 2022-2024, Pulumi Corporation.
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

package batchyaml

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	codegenGo "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// This specifically tests the synced examples from pulumi/yaml with
// testing/test/testdata/transpiled_examples, as it requires a different SDK path in Check
func TestGenerateProgram(t *testing.T) {
	t.Parallel()

	sdkDir, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "sdk"))
	assert.NoError(t, err)

	err = os.Chdir("../../../go") // chdir into codegen/go
	assert.NoError(t, err)

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				codegenGo.Check(t, path, dependencies, sdkDir)
			},
			GenProgram: func(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
				// Prevent tests from interfering with each other
				return codegenGo.GenerateProgramWithOptions(program,
					codegenGo.GenerateProgramOptions{ExternalCache: codegenGo.NewCache()})
			},
			TestCases: test.PulumiPulumiYAMLProgramTests,
		})
}
