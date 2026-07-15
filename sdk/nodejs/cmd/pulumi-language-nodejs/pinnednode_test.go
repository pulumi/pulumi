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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3/noderesolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrependPathInEnv(t *testing.T) {
	t.Parallel()

	sep := string(os.PathListSeparator)

	env := []string{"FOO=bar", "PATH=/usr/bin" + sep + "/bin"}
	got := prependPathInEnv(env, "/managed/bin")
	assert.Equal(t, []string{"FOO=bar", "PATH=/managed/bin" + sep + "/usr/bin" + sep + "/bin"}, got)
	assert.Equal(t, "PATH=/usr/bin"+sep+"/bin", env[1], "input must not be mutated")

	// Windows spells it "Path"; the key match is case-insensitive and the
	// original key casing is preserved.
	got = prependPathInEnv([]string{"Path=C:\\Windows"}, "C:\\managed")
	assert.Equal(t, []string{"Path=C:\\managed" + sep + "C:\\Windows"}, got)

	got = prependPathInEnv([]string{"FOO=bar"}, "/managed/bin")
	assert.Equal(t, []string{"FOO=bar", "PATH=/managed/bin"}, got)
}

func TestPinnedNpmInstall(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("test fakes npm with a shell script")
	}

	binDir := t.TempDir()
	marker := filepath.Join(binDir, "invoked")
	script := "#!/bin/sh\necho \"$@\" > " + marker + "\necho \"PATH=$PATH\" >> " + marker + "\n"
	npmPath := filepath.Join(binDir, "npm")
	//nolint:gosec // we want this file to be executable
	require.NoError(t, os.WriteFile(npmPath, []byte(script), 0o755))

	dir := t.TempDir()
	node := noderesolver.Result{Node: filepath.Join(binDir, "node"), Npm: npmPath, BinDir: binDir}
	var out bytes.Buffer
	require.NoError(t, pinnedNpmInstall(t.Context(), node, dir, false, &out, &out))

	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Contains(t, string(data), "install --loglevel=error")
	assert.Contains(t, string(data), "PATH="+binDir)

	require.NoError(t, pinnedNpmInstall(t.Context(), node, dir, true, &out, &out))
	data, err = os.ReadFile(marker)
	require.NoError(t, err)
	assert.Contains(t, string(data), "--production")
}
