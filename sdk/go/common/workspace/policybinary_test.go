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
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("#!"), 0o755)) //nolint:gosec
}

func TestPolicyBinaryConventionPlatform(t *testing.T) {
	t.Parallel()

	platform, ok := PolicyBinaryConventionPlatform("pulumi-analyzer-mypack-linux-amd64")
	require.True(t, ok)
	assert.Equal(t, "linux-amd64", platform)

	platform, ok = PolicyBinaryConventionPlatform("pulumi-analyzer-mypack-windows-amd64.exe")
	require.True(t, ok)
	assert.Equal(t, "windows-amd64", platform)

	_, ok = PolicyBinaryConventionPlatform("pulumi-analyzer-mypack")
	assert.False(t, ok, "no platform suffix")

	_, ok = PolicyBinaryConventionPlatform("pulumi-analyzer-mypack-freebsd-riscv")
	assert.False(t, ok, "unknown platform")

	_, ok = PolicyBinaryConventionPlatform("index.js")
	assert.False(t, ok, "no policy binary prefix")
}

func TestParsePolicyBinaryOverrides(t *testing.T) {
	t.Parallel()

	m, err := ParsePolicyBinaryOverrides([]string{
		"linux-amd64=out/a",
		"darwin-arm64=out/b",
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"linux-amd64": "out/a", "darwin-arm64": "out/b"}, m)

	_, err = ParsePolicyBinaryOverrides([]string{"freebsd-riscv=out/a"})
	require.ErrorContains(t, err, "freebsd-riscv")

	_, err = ParsePolicyBinaryOverrides([]string{"linux-amd64"})
	require.ErrorContains(t, err, "expected <os>-<arch>=<path>")

	_, err = ParsePolicyBinaryOverrides([]string{"linux-amd64=/abs/path"})
	require.ErrorContains(t, err, "relative")
}

func TestPolicyPackBinaryCanonical(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := "pulumi-analyzer-mypack"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	touch(t, filepath.Join(dir, name))

	bin, ok := PolicyPackBinary(dir)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(dir, name), bin)
}

func TestPolicyPackBinaryConventionFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := "pulumi-analyzer-mypack-" + CurrentPlatform()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	touch(t, filepath.Join(dir, "bin", name))

	bin, ok := PolicyPackBinary(dir)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(dir, "bin", name), bin)
}

func TestPolicyPackBinaryAbsent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "index.ts"))

	_, ok := PolicyPackBinary(dir)
	assert.False(t, ok)
}

func TestPolicyPackBinaryAmbiguous(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "pulumi-analyzer-a"))
	touch(t, filepath.Join(dir, "pulumi-analyzer-b"))

	_, ok := PolicyPackBinary(dir)
	assert.False(t, ok)
}
