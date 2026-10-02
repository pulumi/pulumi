// Copyright 2016, Pulumi Corporation.
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

package packagecmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetermineNPMTagFromCommandResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		currentVersion string
		npmOutput      string
		npmStderr      string
		npmError       error
		expectedTag    string
		expectedErr    string
	}{
		{
			name:           "package doesn't exist - 404 error with empty output",
			currentVersion: "1.0.0",
			npmOutput:      "",
			npmStderr:      "npm error code E404\nnpm error 404 Not Found",
			npmError:       errors.New("command failed"),
			expectedTag:    "latest",
		},
		{
			name:           "package doesn't exist - different 404 message",
			currentVersion: "1.0.0",
			npmOutput:      "",
			npmStderr:      "npm error 404 The requested resource could not be found",
			npmError:       errors.New("command failed"),
			expectedTag:    "latest",
		},
		{
			name:           "current version greater than latest",
			currentVersion: "2.0.0",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       nil,
			expectedTag:    "latest",
		},
		{
			name:           "current version equal to latest",
			currentVersion: "1.0.0",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       nil,
			expectedTag:    "latest",
		},
		{
			name:           "current version less than latest - backport",
			currentVersion: "1.0.0",
			npmOutput:      "2.0.0",
			npmStderr:      "",
			npmError:       nil,
			expectedTag:    "backport",
		},
		{
			name:           "patch version less than latest - backport",
			currentVersion: "1.0.0",
			npmOutput:      "1.0.1",
			npmStderr:      "",
			npmError:       nil,
			expectedTag:    "backport",
		},
		{
			name:           "minor version less than latest - backport",
			currentVersion: "1.0.0",
			npmOutput:      "1.1.0",
			npmStderr:      "",
			npmError:       nil,
			expectedTag:    "backport",
		},
		{
			name:           "patch version greater than latest",
			currentVersion: "1.0.1",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       nil,
			expectedTag:    "latest",
		},
		{
			name:           "invalid current version",
			currentVersion: "not-a-version",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       nil,
			expectedErr:    "failed to parse current version",
		},
		{
			name:           "invalid npm version output",
			currentVersion: "1.0.0",
			npmOutput:      "not-a-version",
			npmStderr:      "",
			npmError:       nil,
			expectedErr:    "failed to parse latest version",
		},
		{
			name:           "non-404 error - should fail",
			currentVersion: "1.0.0",
			npmOutput:      "",
			npmStderr:      "npm error network timeout",
			npmError:       errors.New("network timeout"),
			expectedErr:    "failed to get latest version from npm",
		},
		{
			name:           "error with output but no 404 - should fail",
			currentVersion: "1.0.0",
			npmOutput:      "some output",
			npmStderr:      "npm error authentication failed",
			npmError:       errors.New("auth failed"),
			expectedErr:    "failed to get latest version from npm",
		},
		{
			name:           "404 error but has output - should fail",
			currentVersion: "1.0.0",
			npmOutput:      "1.0.0",
			npmStderr:      "npm error 404",
			npmError:       errors.New("command failed"),
			expectedErr:    "failed to get latest version from npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tag, err := determineNPMTagFromCommandResult(tt.currentVersion, tt.npmOutput, tt.npmStderr, tt.npmError)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTag, tag)
			}
		})
	}
}

func TestShouldRunNPMWhoami(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		nodeAuthToken            string
		actionsIDTokenRequestURL string
		expected                 bool
	}{
		{
			name:     "tokenless non-OIDC path still runs whoami",
			expected: true,
		},
		{
			name:                     "token path still runs whoami in Actions OIDC",
			nodeAuthToken:            "npm_token",
			actionsIDTokenRequestURL: "https://pipelines.actions.githubusercontent.com/example",
			expected:                 true,
		},
		{
			name:                     "tokenless Actions OIDC skips whoami",
			actionsIDTokenRequestURL: "https://pipelines.actions.githubusercontent.com/example",
			expected:                 false,
		},
		{
			name:          "token without Actions OIDC runs whoami",
			nodeAuthToken: "npm_token",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, shouldRunNPMWhoami(tt.nodeAuthToken, tt.actionsIDTokenRequestURL))
		})
	}
}

func TestPublishToNPMSkipsWhoamiForOIDC(t *testing.T) {
	tmp := t.TempDir()
	pkgDir := filepath.Join(tmp, "pkg")
	require.NoError(t, os.Mkdir(pkgDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{
		"name": "@pulumi/test-package",
		"version": "1.0.0-alpha.1"
	}`), 0o600))

	binDir := filepath.Join(tmp, "bin")
	require.NoError(t, os.Mkdir(binDir, 0o700))
	recordFile := filepath.Join(tmp, "npm-calls")
	writeTestNPM(t, binDir, `
package main

import (
	"fmt"
	"os"
)

func main() {
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	f, err := os.OpenFile(`+strconv.Quote(recordFile)+`, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	fmt.Fprintln(f, arg)

	switch arg {
	case "whoami":
		fmt.Fprintln(os.Stderr, "whoami should not be called")
		os.Exit(1)
	case "info":
		os.Exit(1)
	case "publish":
		os.Exit(0)
	}
}
`)
	t.Setenv("GOPATH", filepath.Join(tmp, "gopath"))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("NODE_AUTH_TOKEN", "")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "https://pipelines.actions.githubusercontent.com/example")

	var stdout, stderr bytes.Buffer
	require.NoError(t, publishToNPM(&stdout, &stderr, pkgDir))

	calls, err := os.ReadFile(recordFile)
	require.NoError(t, err)
	assert.NotContains(t, string(calls), "whoami")
	assert.Contains(t, string(calls), "publish")
}

func writeTestNPM(t *testing.T, binDir, source string) {
	t.Helper()

	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte(source), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "go.mod"), []byte("module test-npm\n\ngo 1.24\n"), 0o600))

	npmName := "npm"
	if runtime.GOOS == "windows" {
		npmName += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, npmName), ".")
	cmd.Dir = sourceDir
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
}

func TestPublishToNPMRunsWhoamiWithToken(t *testing.T) {
	tmp := t.TempDir()
	pkgDir := filepath.Join(tmp, "pkg")
	require.NoError(t, os.Mkdir(pkgDir, 0o700))

	binDir := filepath.Join(tmp, "bin")
	require.NoError(t, os.Mkdir(binDir, 0o700))
	writeTestNPM(t, binDir, `
package main

import (
	"fmt"
	"os"
)

func main() {
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	if arg == "whoami" {
		fmt.Fprintln(os.Stderr, "token auth failed")
		os.Exit(42)
	}
	fmt.Fprintf(os.Stderr, "unexpected npm command: %s\n", arg)
	os.Exit(1)
}
`)
	t.Setenv("GOPATH", filepath.Join(tmp, "gopath"))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("NODE_AUTH_TOKEN", "npm_token")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "https://pipelines.actions.githubusercontent.com/example")

	var stdout, stderr bytes.Buffer
	err := publishToNPM(&stdout, &stderr, pkgDir)
	require.Error(t, err)
	assert.Contains(t, stderr.String(), "token auth failed")
}

func TestDetermineNPMTagForStableVersion(t *testing.T) {
	t.Parallel()
	_, err := executable.FindExecutable("npm")
	if err != nil {
		t.Skip("could not find npm; skipping integration test")
	}

	// Test with a real package that exists
	tag, err := determineNPMTagForStableVersion("npm", "@pulumi/aws", "5.0.0")
	require.NoError(t, err)
	assert.Contains(t, "backport", tag)

	// Test with a package that doesn't exist
	tag, err = determineNPMTagForStableVersion("npm", "@pulumi/nonexistent-package-12345", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "latest", tag)

	// Test with a package name that is higher semver.
	// Using v100 as major version to avoid tripping...for the next 97 major releases.
	tag, err = determineNPMTagForStableVersion("npm", "@pulumi/aws", "100.0.0")
	require.NoError(t, err)
	assert.Equal(t, "latest", tag)
}
