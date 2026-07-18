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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindAnalyzerBinary(t *testing.T) {
	t.Parallel()

	binName := "pulumi-analyzer-mypack"
	if strings.HasPrefix(CurrentPlatform(), "windows-") {
		binName += ".exe"
	}

	t.Run("binary at root is found", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		bin := filepath.Join(dir, binName)
		require.NoError(t, os.WriteFile(bin, []byte("bin"), 0o755)) //nolint:gosec
		got, ok := FindAnalyzerBinary(dir)
		require.True(t, ok)
		assert.Equal(t, bin, got)
	})

	t.Run("no binary", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("//"), 0o600))
		_, ok := FindAnalyzerBinary(dir)
		require.False(t, ok)
	})

	t.Run("binary in subdir is not discovered", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "bin"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "bin", binName), []byte("bin"), 0o755)) //nolint:gosec
		_, ok := FindAnalyzerBinary(dir)
		require.False(t, ok)
	})

	t.Run("directory named like the binary is skipped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "pulumi-analyzer-notabinary"), 0o755))
		_, ok := FindAnalyzerBinary(dir)
		require.False(t, ok)
	})
}

// otherPlatform returns a valid policy binary platform other than current, with the same
// windows-ness so its filename carries (or omits) the ".exe" suffix the same way.
func otherPlatform(current string) string {
	windows := strings.HasPrefix(current, "windows-")
	for p := range ValidPolicyBinaryPlatforms {
		if p != current && strings.HasPrefix(p, "windows-") == windows {
			return p
		}
	}
	return current
}

// platformBinName returns the cross-compiled build-directory filename for a platform:
// pulumi-analyzer-<name>-<os>-<arch>[.exe].
func platformBinName(name, platform string) string {
	n := "pulumi-analyzer-" + name + "-" + platform
	if strings.HasPrefix(platform, "windows-") {
		n += ".exe"
	}
	return n
}

func writeExecFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("bin"), 0o755)) //nolint:gosec
}

func TestFindAnalyzerBinaryPlatformAware(t *testing.T) {
	t.Parallel()

	current := CurrentPlatform()
	other := otherPlatform(current)

	t.Run("prefers the current-platform binary in a matrix directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExecFile(t, filepath.Join(dir, platformBinName("mypack", current)))
		writeExecFile(t, filepath.Join(dir, platformBinName("mypack", other)))
		got, ok := FindAnalyzerBinary(dir)
		require.True(t, ok)
		assert.Equal(t, filepath.Join(dir, platformBinName("mypack", current)), got)
	})

	t.Run("a binary only for another platform is not runnable", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExecFile(t, filepath.Join(dir, platformBinName("mypack", other)))
		_, ok := FindAnalyzerBinary(dir)
		require.False(t, ok)
	})
}

func TestResolveAnalyzerBinary(t *testing.T) {
	t.Parallel()

	// The bare installed-artifact name for this platform (".exe" on Windows).
	bareName := AnalyzerBinaryName("mypack", CurrentPlatform())

	t.Run("direct executable file is used as-is", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		bin := filepath.Join(dir, platformBinName("mypack", CurrentPlatform()))
		writeExecFile(t, bin)
		gotBin, gotDir, ok := ResolveAnalyzerBinary(bin)
		require.True(t, ok)
		assert.Equal(t, bin, gotBin)
		assert.Equal(t, dir, gotDir)
	})

	t.Run("directory containing a binary", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		bin := filepath.Join(dir, bareName)
		writeExecFile(t, bin)
		gotBin, gotDir, ok := ResolveAnalyzerBinary(dir)
		require.True(t, ok)
		assert.Equal(t, bin, gotBin)
		assert.Equal(t, dir, gotDir)
	})

	t.Run("directory without a binary", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("//"), 0o600))
		_, gotDir, ok := ResolveAnalyzerBinary(dir)
		require.False(t, ok)
		assert.Equal(t, dir, gotDir)
	})
}
