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

package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePolicyBinaries(t *testing.T) {
	t.Parallel()

	require.NoError(t, validatePolicyBinaries(nil))
	require.NoError(t, validatePolicyBinaries(map[string]string{
		"linux-amd64":   "bin/pulumi-analyzer-mypack-linux-amd64",
		"windows-amd64": "bin/pulumi-analyzer-mypack-windows-amd64.exe",
	}))

	err := validatePolicyBinaries(map[string]string{"freebsd-riscv": "bin/a"})
	require.ErrorContains(t, err, "freebsd-riscv")

	err = validatePolicyBinaries(map[string]string{"linux-amd64": ""})
	require.ErrorContains(t, err, "missing a path")

	err = validatePolicyBinaries(map[string]string{"linux-amd64": "/abs/path"})
	require.ErrorContains(t, err, "relative")

	err = validatePolicyBinaries(map[string]string{"linux-amd64": `C:\abs\path`})
	require.ErrorContains(t, err, "relative")

	err = validatePolicyBinaries(map[string]string{"linux-amd64": `\abs\path`})
	require.ErrorContains(t, err, "relative")
}

func TestPolicyPackProjectBinaryRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "PulumiPolicy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(
		"runtime: nodejs\nbinary:\n  linux-amd64: bin/pulumi-analyzer-mypack-linux-amd64\n"), 0o600))

	proj, err := LoadPolicyPack(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"linux-amd64": "bin/pulumi-analyzer-mypack-linux-amd64",
	}, proj.Binary)

	require.NoError(t, proj.Save(path))
	reloaded, err := LoadPolicyPack(path)
	require.NoError(t, err)
	assert.Equal(t, proj.Binary, reloaded.Binary)
}

func TestPolicyPackProjectBinaryValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "PulumiPolicy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(
		"runtime: nodejs\nbinary:\n  freebsd-riscv: bin/a\n"), 0o600))

	_, err := LoadPolicyPack(path)
	require.ErrorContains(t, err, "freebsd-riscv")
}
