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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	codegenNode "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
)

// This specifically tests the synced examples from pulumi/yaml with
// testing/test/testdata/transpiled_examples
func TestGenerateProgram(t *testing.T) {
	t.Parallel()
	err := os.Chdir("../../../nodejs") // chdir into codegen/nodejs
	assert.NoError(t, err)

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "nodejs",
			Extension:  "ts",
			OutputFile: "index.ts",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				codegenNode.Check(t, path, dependencies, true)
			},
			GenProgram: codegenNode.GenerateProgram,
			TestCases:  test.PulumiPulumiYAMLProgramTests,
		})
}
