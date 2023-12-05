// Copyright 2016-2021, Pulumi Corporation.
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

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that RunCommand writes the command's output to a log file.
func TestRunCommandLog(t *testing.T) {
	t.Parallel()

	// Try to find node on the path. We need a program to run, and node is probably
	// available on all platforms where we're testing. If it's not found, skip the test.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Couldn't find Node on PATH")
	}

	testw := iotest.LogWriter(t)
	opts := &ProgramTestOptions{
		Stdout: testw,
		Stderr: testw,
	}

	tempdir := t.TempDir()

	args := []string{node, "-e", "console.log('output from node');"}
	err = RunCommand(t, "node", args, tempdir, opts)
	assert.NoError(t, err)

	matches, err := filepath.Glob(filepath.Join(tempdir, commandOutputFolderName, "node.*"))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(matches))

	output, err := os.ReadFile(matches[0])
	assert.NoError(t, err)
	assert.Equal(t, "output from node\n", string(output))
}

func TestSanitizedPkg(t *testing.T) {
	t.Parallel()

	v2 := getSanitizedModulePath("github.com/pulumi/pulumi-docker/sdk/v2")
	assert.Equal(t, "github.com/pulumi/pulumi-docker/sdk", v2)

	v3 := getSanitizedModulePath("github.com/pulumi/pulumi-aws/sdk/v3")
	assert.Equal(t, "github.com/pulumi/pulumi-aws/sdk", v3)

	nonVersion := getSanitizedModulePath("github.com/pulumi/pulumi-auth/sdk")
	assert.Equal(t, "github.com/pulumi/pulumi-auth/sdk", nonVersion)
}

func TestDepRootCalc(t *testing.T) {
	t.Parallel()

	var dep string

	dep = getRewritePath("github.com/pulumi/pulumi-docker/sdk/v2", "/gopath", "")
	assert.Equal(t, "/gopath/src/github.com/pulumi/pulumi-docker/sdk", filepath.ToSlash(dep))

	dep = getRewritePath("github.com/pulumi/pulumi-gcp/sdk/v3", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/pulumi-gcp/sdk", filepath.ToSlash(dep))

	dep = getRewritePath("github.com/example/foo/pkg/v2", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/foo/pkg", filepath.ToSlash(dep))

	dep = getRewritePath("github.com/example/foo/v2", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/foo", filepath.ToSlash(dep))

	dep = getRewritePath("github.com/example/foo", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/foo", filepath.ToSlash(dep))

	dep = getRewritePath("github.com/pulumi/pulumi-auth0/sdk", "gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/pulumi-auth0/sdk", filepath.ToSlash(dep))
}

func TestGoModEdits(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)
	depRoot := filepath.Clean(filepath.Join(cwd, "../../../.."))

	gopath, err := GoPath()
	require.NoError(t, err)

	// Were we to commit this go.mod file, `make tidy` would fail, and we should keep the complexity
	// of tests constrained to the test itself.

	// The dir must be a relative path as well, so we make it relative to cwd (which is absolute).
	badModDir := t.TempDir()
	badModDir, err = filepath.Rel(cwd, badModDir)
	require.NoError(t, err)
	badModFile := filepath.Join(badModDir, "go.mod")
	err = os.WriteFile(badModFile, []byte(`
# invalid go.mod
`), 0o600)
	require.NoError(t, err)

	errNotExists := "no such file or directory"
	if runtime.GOOS == "windows" {
		errNotExists = "The system cannot find the path specified"
	}

	tests := []struct {
		name          string
		dep           string
		expectedValue string
		expectedError string
	}{
		{
			name:          "valid-path",
			dep:           "../../../sdk",
			expectedValue: "github.com/pulumi/pulumi/sdk/v3=" + filepath.Join(cwd, "../../../sdk"),
		},
		{
			name:          "invalid-path-non-existent",
			dep:           "../../../.tmp.non-existent-dir",
			expectedError: errNotExists,
		},
		{
			name:          "invalid-path-bad-go-mod",
			dep:           badModDir,
			expectedError: "error parsing go.mod",
		},
		{
			name:          "valid-module-name",
			dep:           "github.com/pulumi/pulumi/sdk/v3",
			expectedValue: "github.com/pulumi/pulumi/sdk/v3=" + filepath.Join(cwd, "../../../sdk"),
		},
		{
			name:          "valid-module-name-version-skew",
			dep:           "github.com/pulumi/pulumi/sdk",
			expectedValue: "github.com/pulumi/pulumi/sdk=" + filepath.Join(cwd, "../../../sdk"),
		},
		{
			name:          "valid-rel-path",
			dep:           "github.com/pulumi/pulumi/sdk/v3=../../../sdk",
			expectedValue: "github.com/pulumi/pulumi/sdk/v3=" + filepath.Join(cwd, "../../../sdk"),
		},
		{
			name:          "valid-rel-path-version-skew",
			dep:           "github.com/pulumi/pulumi/sdk=../../../sdk",
			expectedValue: "github.com/pulumi/pulumi/sdk=" + filepath.Join(cwd, "../../../sdk"),
		},
		{
			name:          "invalid-rel-path",
			dep:           "github.com/pulumi/pulumi/pkg=../../../sdk",
			expectedError: "found module path with prefix github.com/pulumi/pulumi/sdk, expected github.com/pulumi/pulumi/pkg",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			editStr, err := getEditStr(test.dep, gopath, depRoot)

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedValue, editStr)
			}
		})
	}
}
