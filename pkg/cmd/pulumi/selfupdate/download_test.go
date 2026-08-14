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

package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checksums is a copy of the format published alongside a release.
const checksums = `a8bb6c8828bb6c716ed1d08cc3d94ca7a2435ed9c7d208705f422529d39614a0  pulumi-v3.257.0-darwin-arm64.tar.gz
d5e2155d59cf65fadedd63d8af28e82b286d94a4c2b80cfaaedd76418b840298  pulumi-v3.257.0-darwin-x64.tar.gz
e5c22687e56f77ac094d4b74c0c48a2de0da699ce72644c9bf173d98980946a6  pulumi-v3.257.0-linux-arm64.tar.gz
a99c21521eb5ac74849f5ea55f46fdc3e93006633d0d3ae6862a2e83325f7a92  pulumi-v3.257.0-windows-x64.zip
`

func TestFindChecksum(t *testing.T) {
	t.Parallel()

	sum, err := findChecksum(checksums, "pulumi-v3.257.0-linux-arm64.tar.gz")

	require.NoError(t, err)
	assert.Equal(t, "e5c22687e56f77ac094d4b74c0c48a2de0da699ce72644c9bf173d98980946a6", sum)
}

func TestFindChecksumReportsMissingArtifact(t *testing.T) {
	t.Parallel()

	_, err := findChecksum(checksums, "pulumi-v3.257.0-plan9-x64.tar.gz")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pulumi-v3.257.0-plan9-x64.tar.gz")
}

func TestArtifactNameMatchesThePublishedNames(t *testing.T) {
	t.Parallel()

	v := semver.MustParse("3.257.0")
	tests := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "pulumi-v3.257.0-darwin-arm64.tar.gz"},
		{"darwin", "amd64", "pulumi-v3.257.0-darwin-x64.tar.gz"},
		{"linux", "arm64", "pulumi-v3.257.0-linux-arm64.tar.gz"},
		{"linux", "amd64", "pulumi-v3.257.0-linux-x64.tar.gz"},
		{"windows", "amd64", "pulumi-v3.257.0-windows-x64.zip"},
		{"windows", "arm64", "pulumi-v3.257.0-windows-arm64.zip"},
	}

	for _, tt := range tests {
		// These are exactly the names listed in the checksums file published with each release.
		got, err := artifactNameFor(v, tt.goos, tt.goarch)
		require.NoError(t, err, "%s/%s", tt.goos, tt.goarch)
		assert.Equal(t, tt.want, got)
	}
}

func TestArtifactNameRejectsUnsupportedPlatforms(t *testing.T) {
	t.Parallel()

	v := semver.MustParse("3.257.0")

	_, err := artifactNameFor(v, "linux", "386")
	assert.ErrorContains(t, err, "unsupported architecture")

	_, err = artifactNameFor(v, "plan9", "amd64")
	assert.ErrorContains(t, err, "unsupported operating system")
}

func TestSHA256File(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "artifact")
	require.NoError(t, os.WriteFile(path, []byte("pulumi"), 0o600))

	sum, err := sha256File(path)

	require.NoError(t, err)
	// echo -n pulumi | shasum -a 256
	assert.Equal(t, "fbe2a04069387628783a3f90b947236e6ff8b1c099e710871356a6381a4e20b2", sum)
}

func TestExtractReadsATarball(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "pulumi-v3.257.0-linux-x64.tar.gz")
	writeTarball(t, path, map[string]string{
		"pulumi/pulumi":                 "the cli",
		"pulumi/pulumi-language-nodejs": "the nodejs plugin",
	})

	staged, err := extract(path, t.TempDir())

	require.NoError(t, err)
	contents, err := os.ReadFile(filepath.Join(staged, "pulumi"))
	require.NoError(t, err)
	assert.Equal(t, "the cli", string(contents))
}

func TestExtractReadsTheOlderBinLayout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "pulumi-v3.257.0-linux-x64.tar.gz")
	writeTarball(t, path, map[string]string{"pulumi/bin/pulumi": "the cli"})

	staged, err := extract(path, t.TempDir())

	require.NoError(t, err)
	assert.Equal(t, "bin", filepath.Base(staged))
	assert.FileExists(t, filepath.Join(staged, "pulumi"))
}

func TestExtractReadsAZip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "pulumi-v3.257.0-windows-x64.zip")
	writeZip(t, path, map[string]string{"pulumi/pulumi.exe": "the cli"})

	staged, err := extract(path, t.TempDir())

	require.NoError(t, err)
	contents, err := os.ReadFile(filepath.Join(staged, "pulumi.exe"))
	require.NoError(t, err)
	assert.Equal(t, "the cli", string(contents))
}

func TestExtractZipRejectsEntriesEscapingTheDestination(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "evil.zip")
	writeZip(t, path, map[string]string{"../escaped": "should not be written"})

	err := extractZip(path, t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the destination directory")
}

func writeTarball(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for name, contents := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o700,
			Size: int64(len(contents)),
		}))
		_, err := tw.Write([]byte(contents))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, contents := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(contents))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
}
