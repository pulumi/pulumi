// Copyright 2022, Pulumi Corporation.
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

package convert

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalStackJSON returns a minimal valid UntypedDeployment JSON containing only a stack root resource
// and a provider resource, both of which are filtered out by the stack converter.
func minimalStackJSON() string {
	return `{
  "version": 3,
  "deployment": {
    "manifest": {
      "time": "2024-01-01T00:00:00Z",
      "magic": "abc123",
      "version": "v3.0.0"
    },
    "resources": [
      {
        "urn": "urn:pulumi:dev::myproject::pulumi:pulumi:Stack::myproject-dev",
        "custom": false,
        "type": "pulumi:pulumi:Stack",
        "inputs": {},
        "outputs": {}
      },
      {
        "urn": "urn:pulumi:dev::myproject::pulumi:providers:random::default",
        "custom": true,
        "type": "pulumi:providers:random",
        "id": "default",
        "inputs": {},
        "outputs": {}
      }
    ]
  }
}`
}

func TestStackConvertRequiresFile(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	cwd, err := filepath.Abs(".")
	require.NoError(t, err)

	err = runConvert(
		t.Context(),
		&cmdBackend.MockLoginManager{},
		pkgWorkspace.Instance,
		env.Global(),
		[]string{},
		cwd,
		[]string{},
		"stack",
		"pcl",
		outDir,
		true,  /*generateOnly*/
		false, /*strict*/
		"myproject",
	)
	require.ErrorContains(t, err, "--file is required when --from stack")
}

func TestStackConvertInvalidJSON(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	cwd, err := filepath.Abs(".")
	require.NoError(t, err)

	badFile := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(badFile, []byte("not valid json"), 0o600))

	err = runConvert(
		t.Context(),
		&cmdBackend.MockLoginManager{},
		pkgWorkspace.Instance,
		env.Global(),
		[]string{"--file", badFile},
		cwd,
		[]string{},
		"stack",
		"pcl",
		outDir,
		true,  /*generateOnly*/
		false, /*strict*/
		"myproject",
	)
	require.ErrorContains(t, err, "parse stack file")
}

func TestStackConvertEmptyStack(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	cwd, err := filepath.Abs(".")
	require.NoError(t, err)

	stackFile := filepath.Join(t.TempDir(), "stack.json")
	require.NoError(t, os.WriteFile(stackFile, []byte(minimalStackJSON()), 0o600))

	err = runConvert(
		t.Context(),
		&cmdBackend.MockLoginManager{},
		pkgWorkspace.Instance,
		env.Global(),
		[]string{"--file", stackFile},
		cwd,
		[]string{},
		"stack",
		"pcl",
		outDir,
		true,  /*generateOnly*/
		false, /*strict*/
		"myproject",
	)
	require.NoError(t, err)

	// The stack root and provider resources are filtered out, so program.pp should be empty.
	pclBytes, err := os.ReadFile(filepath.Join(outDir, "program.pp"))
	require.NoError(t, err)
	assert.Empty(t, pclBytes)
}

// TestStackConvertFileNotFound verifies the error when the stack file cannot be read.
func TestStackConvertFileNotFound(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	cwd, err := filepath.Abs(".")
	require.NoError(t, err)

	err = runConvert(
		t.Context(),
		&cmdBackend.MockLoginManager{},
		pkgWorkspace.Instance,
		env.Global(),
		[]string{"--file", filepath.Join(t.TempDir(), "nonexistent.json")},
		cwd,
		[]string{},
		"stack",
		"pcl",
		outDir,
		true,  /*generateOnly*/
		false, /*strict*/
		"myproject",
	)
	require.ErrorContains(t, err, "read stack file")
}

// TestYamlConvert is an entrypoint for debugging `pulumi convert`. To use this with an editor such as
// VS Code, drop a Pulumi.yaml in the testdata folder and with the VS Code Go extension, the
// code lens (grayed out text above TestConvert) should display an option to "debug test".
//
// This is ideal for debugging panics in the convert command, as the debugger will break on the
// panic.
//
// See: https://github.com/golang/vscode-go/wiki/debugging
//
// Your mileage may vary with other tooling.
func TestYamlConvert(t *testing.T) {
	t.Parallel()

	if info, err := os.Stat("testdata/Pulumi.yaml"); err != nil && os.IsNotExist(err) {
		t.Skip("skipping test, no Pulumi.yaml found")
	} else if err != nil {
		t.Fatalf("failed to stat Pulumi.yaml: %v", err)
	} else if info.IsDir() {
		t.Fatalf("Pulumi.yaml is a directory, not a file")
	}

	cwd, err := filepath.Abs("testdata")
	require.NoError(t, err)

	result := runConvert(
		t.Context(), &cmdBackend.MockLoginManager{}, pkgWorkspace.Instance, env.Global(), []string{}, cwd, []string{},
		"yaml", "go", "testdata/go", true, true, "")
	require.Nil(t, result, "convert failed: %v", result)
}

func TestPclConvert(t *testing.T) {
	t.Parallel()

	// Check that we can run convert from PCL to PCL
	tmp := t.TempDir()

	cwd, err := filepath.Abs("pcl_testdata")
	require.NoError(t, err)

	result := runConvert(
		t.Context(), &cmdBackend.MockLoginManager{}, pkgWorkspace.Instance, env.Global(), []string{}, cwd,
		[]string{}, "pcl", "pcl", tmp, true, true, "")
	assert.Nil(t, result)

	// Check that we made one file
	pclBytes, err := os.ReadFile(filepath.Join(tmp, "main.pp"))
	require.NoError(t, err)
	// On Windows, we need to replace \r\n with \n to match the expected string below
	pclCode := string(pclBytes)
	if runtime.GOOS == "windows" {
		pclCode = strings.ReplaceAll(pclCode, "\r\n", "\n")
	}
	expectedPclCode := `key = readFile("key.pub")

output result {
    __logicalName = "result"
    value = key
}`
	assert.Equal(t, expectedPclCode, pclCode)
}

// Tests that project names default to the directory of the source project.
func TestProjectNameDefaults(t *testing.T) {
	t.Parallel()

	// Arrange.
	outDir := t.TempDir()

	cwd, err := filepath.Abs("pcl_testdata")
	require.NoError(t, err)

	// Act.
	err = runConvert(
		t.Context(),
		&cmdBackend.MockLoginManager{},
		pkgWorkspace.Instance,
		env.Global(),
		[]string{}, /*args*/
		cwd,        /*cwd*/
		[]string{}, /*mappings*/
		"pcl",      /*from*/
		"yaml",     /*language*/
		outDir,
		true, /*generateOnly*/
		true, /*strict*/
		"",   /*name*/
	)
	require.NoError(t, err)

	// Assert.
	yamlBytes, err := os.ReadFile(filepath.Join(outDir, "Pulumi.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlBytes), "name: pcl_testdata")
}

// Tests that project names can be overridden by the user.
func TestProjectNameOverrides(t *testing.T) {
	t.Parallel()

	// Arrange.
	outDir := t.TempDir()
	name := "test-project-name"

	cwd, err := filepath.Abs("pcl_testdata")
	require.NoError(t, err)

	// Act.
	err = runConvert(
		t.Context(),
		&cmdBackend.MockLoginManager{},
		pkgWorkspace.Instance,
		env.Global(),
		[]string{}, /*args*/
		cwd,        /*cwd*/
		[]string{}, /*mappings*/
		"pcl",      /*from*/
		"yaml",     /*language*/
		outDir,
		true, /*generateOnly*/
		true, /*strict*/
		name,
	)
	require.NoError(t, err)

	// Assert.
	yamlBytes, err := os.ReadFile(filepath.Join(outDir, "Pulumi.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlBytes), "name: "+name)
}
