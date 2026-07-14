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
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestPack(t *testing.T, binaries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PulumiPolicy.yaml"),
		[]byte("runtime: nodejs\n"), 0o600))
	for _, rel := range binaries {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("bin"), 0o755))
	}
	return dir
}

func TestValidateBinaryMatrixRequiresLinuxAmd64(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{"darwin-arm64": filepath.Join("bin", "b")}
	dir := writeTestPack(t, binaries)
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, workspace.PlatformLinuxAmd64)
}

func TestValidateBinaryMatrixRequiresHostPlatform(t *testing.T) {
	t.Parallel()

	if workspace.CurrentPlatform() == workspace.PlatformLinuxAmd64 {
		t.Skip("host platform is linux-amd64; the mandatory-platform check subsumes this")
	}
	binaries := map[string]string{workspace.PlatformLinuxAmd64: filepath.Join("bin", "b")}
	dir := writeTestPack(t, binaries)
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, workspace.CurrentPlatform())
}

func TestValidateBinaryMatrixMissingFile(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{
		workspace.PlatformLinuxAmd64: filepath.Join("bin", "b"),
		workspace.CurrentPlatform():  filepath.Join("bin", "host"),
	}
	dir := writeTestPack(t, binaries)
	require.NoError(t, os.Remove(filepath.Join(dir, "bin", "b")))
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, filepath.Join("bin", "b"))
}

func tarEntries(t *testing.T, tgz []byte) map[string]int64 {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	entries := map[string]int64{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Typeflag == tar.TypeReg {
			entries[filepath.ToSlash(hdr.Name)] = hdr.Mode
		}
	}
	return entries
}

func TestBuildBinaryArtifact(t *testing.T) {
	t.Parallel()

	rel := filepath.Join("bin", "pulumi-analyzer-mypack-linux-amd64")
	dir := writeTestPack(t, map[string]string{"linux-amd64": rel})

	tgz, err := buildBinaryArtifact(dir, rel, "mypack", "linux-amd64")
	require.NoError(t, err)

	entries := tarEntries(t, tgz)
	assert.Contains(t, entries, "package/PulumiPolicy.yaml")
	assert.Contains(t, entries, "package/pulumi-analyzer-mypack")
	assert.Len(t, entries, 2)
}

func TestBuildBinaryArtifactWindowsSuffix(t *testing.T) {
	t.Parallel()

	rel := filepath.Join("bin", "pulumi-analyzer-mypack-windows-amd64.exe")
	dir := writeTestPack(t, map[string]string{"windows-amd64": rel})

	tgz, err := buildBinaryArtifact(dir, rel, "mypack", "windows-amd64")
	require.NoError(t, err)

	entries := tarEntries(t, tgz)
	assert.Contains(t, entries, "package/pulumi-analyzer-mypack.exe")
}
