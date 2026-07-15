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

package httpstate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"maps"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeBinaryDir creates a directory of pre-built analyzer binaries named
// pulumi-analyzer-<name>-<platform>[.exe], one per given platform, and returns the dir.
func writeBinaryDir(t *testing.T, name string, platforms ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, platform := range platforms {
		binName := "pulumi-analyzer-" + name + "-" + platform
		if strings.HasPrefix(platform, "windows-") {
			binName += ".exe"
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, binName), []byte("bin"), 0o755)) //nolint:gosec
	}
	return dir
}

func TestDiscoverPolicyBinaries(t *testing.T) {
	t.Parallel()

	dir := writeBinaryDir(t, "mypack", "linux-amd64", "darwin-arm64", "windows-amd64")
	// Non-analyzer files and subdirectories must be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pulumi-analyzer-notabinary"), 0o755))

	binaries, err := discoverPolicyBinaries(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"darwin-arm64", "linux-amd64", "windows-amd64"},
		slices.Sorted(maps.Keys(binaries)))
	assert.Equal(t, filepath.Join(dir, "pulumi-analyzer-mypack-linux-amd64"), binaries["linux-amd64"])
	assert.Equal(t, filepath.Join(dir, "pulumi-analyzer-mypack-windows-amd64.exe"), binaries["windows-amd64"])
}

func TestDiscoverPolicyBinariesEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("//"), 0o600))
	binaries, err := discoverPolicyBinaries(dir)
	require.NoError(t, err)
	assert.Empty(t, binaries)
}

func TestValidateBinaryMatrixRequiresLinuxAmd64(t *testing.T) {
	t.Parallel()

	binaries, err := discoverPolicyBinaries(writeBinaryDir(t, "mypack", "darwin-arm64"))
	require.NoError(t, err)
	require.ErrorContains(t, validateBinaryMatrix(binaries), platformLinuxAmd64)
}

func TestValidateBinaryMatrixRequiresHostPlatform(t *testing.T) {
	t.Parallel()

	if workspace.CurrentPlatform() == platformLinuxAmd64 {
		t.Skip("host platform is linux-amd64; the mandatory-platform check subsumes this")
	}
	binaries, err := discoverPolicyBinaries(writeBinaryDir(t, "mypack", platformLinuxAmd64))
	require.NoError(t, err)
	require.ErrorContains(t, validateBinaryMatrix(binaries), workspace.CurrentPlatform())
}

func TestValidateBinaryMatrixMissingFile(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{
		platformLinuxAmd64:          filepath.Join(t.TempDir(), "missing-linux"),
		workspace.CurrentPlatform(): filepath.Join(t.TempDir(), "missing-host"),
	}
	require.ErrorContains(t, validateBinaryMatrix(binaries), "was not found")
}

func tarEntries(t *testing.T, tgz []byte) map[string][]byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	entries := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			require.NoError(t, err)
			entries[filepath.ToSlash(hdr.Name)] = content
		}
	}
	return entries
}

func TestBuildBinaryArtifact(t *testing.T) {
	t.Parallel()

	dir := writeBinaryDir(t, "mypack", "linux-amd64")
	binPath := filepath.Join(dir, "pulumi-analyzer-mypack-linux-amd64")

	tgz, err := buildBinaryArtifact(binPath, "mypack", "linux-amd64")
	require.NoError(t, err)

	entries := tarEntries(t, tgz)
	// The artifact is a bare exe — just the canonically named binary, no manifest, so
	// consumers dispatch to it by convention like a provider plugin.
	require.Len(t, entries, 1)
	assert.Contains(t, entries, "package/pulumi-analyzer-mypack")
	assert.NotContains(t, entries, "package/PulumiPolicy.yaml")
}

func TestBuildBinaryArtifactWindowsSuffix(t *testing.T) {
	t.Parallel()

	dir := writeBinaryDir(t, "mypack", "windows-amd64")
	binPath := filepath.Join(dir, "pulumi-analyzer-mypack-windows-amd64.exe")

	tgz, err := buildBinaryArtifact(binPath, "mypack", "windows-amd64")
	require.NoError(t, err)

	entries := tarEntries(t, tgz)
	assert.Contains(t, entries, "package/pulumi-analyzer-mypack.exe")
}

func buildTGZ(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestSourceTarballBinariesDetectsDeclaredPaths(t *testing.T) {
	t.Parallel()

	declared := map[string]bool{
		"bin/pulumi-analyzer-mypack-linux-amd64":       true,
		"bin/pulumi-analyzer-mypack-windows-amd64.exe": true,
		"bin/pulumi-analyzer-mypack-darwin-arm64":      true,
	}
	tgz := buildTGZ(t, map[string]string{
		"package/PulumiPolicy.yaml":                            "runtime: nodejs\n",
		"package/bin/pulumi-analyzer-mypack-linux-amd64":       "bin",
		"package/bin/pulumi-analyzer-mypack-windows-amd64.exe": "bin",
		"package/index.js":                                     "// not a binary",
	})

	found := sourceTarballBinaries(tgz, declared)
	assert.Equal(t, []string{
		"bin/pulumi-analyzer-mypack-linux-amd64",
		"bin/pulumi-analyzer-mypack-windows-amd64.exe",
	}, found)
}

func TestSourceTarballBinariesNoneFound(t *testing.T) {
	t.Parallel()

	declared := map[string]bool{"bin/pulumi-analyzer-mypack-linux-amd64": true}
	tgz := buildTGZ(t, map[string]string{
		"package/PulumiPolicy.yaml": "runtime: nodejs\n",
		"package/index.js":          "// not a binary",
	})

	assert.Empty(t, sourceTarballBinaries(tgz, declared))
}

func TestInstallRequiredPolicySkipsDependenciesForBinaryPack(t *testing.T) {
	t.Parallel()

	platform := workspace.CurrentPlatform()
	binName := "pulumi-analyzer-mypack-" + platform
	if goruntime.GOOS == "windows" {
		binName += ".exe"
	}
	dir := writeBinaryDir(t, "mypack", platform)
	tgz, err := buildBinaryArtifact(filepath.Join(dir, binName), "mypack", platform)
	require.NoError(t, err)

	finalDir := filepath.Join(t.TempDir(), "installed")
	// ctx is never used past the binary short-circuit: a nil-host context proves no
	// language runtime was resolved.
	err = installRequiredPolicy(nil, finalDir, io.NopCloser(bytes.NewReader(tgz)), io.Discard, io.Discard)
	require.NoError(t, err)

	// The installed artifact is a bare exe — no manifest is written or needed.
	_, err = os.Stat(filepath.Join(finalDir, "PulumiPolicy.yaml"))
	require.ErrorIs(t, err, os.ErrNotExist)

	info, err := os.Stat(filepath.Join(finalDir, workspace.AnalyzerBinaryName("mypack", platform)))
	require.NoError(t, err)
	if goruntime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o111)
	}
}
