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

package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

func Check(t *testing.T, path string, deps codegen.StringSet, pulumiSDKPath string) {
	dir := filepath.Dir(path)
	ex, err := executable.FindExecutable("go")
	require.NoError(t, err)

	// We remove go.mod to ensure tests are reproducible.
	goMod := filepath.Join(dir, "go.mod")
	if err = os.Remove(goMod); !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	err = integration.RunCommand(t, "generate go.mod",
		[]string{ex, "mod", "init", "main"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	err = integration.RunCommand(t, "go tidy",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	if pulumiSDKPath != "" {
		err = integration.RunCommand(t, "point towards local Go SDK",
			[]string{
				ex, "mod", "edit",
				fmt.Sprintf("--replace=%s=%s",
					"github.com/pulumi/pulumi/sdk/v3",
					pulumiSDKPath),
			},
			dir, &integration.ProgramTestOptions{})
		require.NoError(t, err)
	}
	TypeCheck(t, path, deps, pulumiSDKPath)
}

func TypeCheck(t *testing.T, path string, deps codegen.StringSet, pulumiSDKPath string) {
	dir := filepath.Dir(path)
	ex, err := executable.FindExecutable("go")
	require.NoError(t, err)

	err = integration.RunCommand(t, "go tidy after replace",
		[]string{ex, "mod", "tidy"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)

	err = integration.RunCommand(t, "test build", []string{ex, "build", "-v", "all"},
		dir, &integration.ProgramTestOptions{})
	require.NoError(t, err)
	os.Remove(filepath.Join(dir, "main"))
	assert.NoError(t, err)
}

func GenerateProgramBatchTest(t *testing.T, testCases []test.ProgramTest) {
	pulumiSdkPath, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk"))
	require.NoError(t, err)
	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "go",
			Extension:  "go",
			OutputFile: "main.go",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, pulumiSdkPath)
			},
			GenProgram: func(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
				// Prevent tests from interfering with each other
				return GenerateProgramWithOptions(program, GenerateProgramOptions{ExternalCache: NewCache()})
			},
			TestCases: testCases,
		})
}
