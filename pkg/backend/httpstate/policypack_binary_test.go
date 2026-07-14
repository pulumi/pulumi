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
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testRuntime = workspace.NewProjectRuntimeInfo("nodejs", nil)

// writeTestPack lays out a pack dir whose manifest declares the given platform-to-path
// binary mapping (slash-separated paths), with placeholder files for each binary.
func writeTestPack(t *testing.T, binaries map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	var manifest strings.Builder
	manifest.WriteString("runtime: nodejs\n")
	if len(binaries) > 0 {
		manifest.WriteString("binary:\n")
		for platform, rel := range binaries {
			fmt.Fprintf(&manifest, "  %s: %s\n", platform, rel)
		}
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PulumiPolicy.yaml"),
		[]byte(manifest.String()), 0o600))

	for _, rel := range binaries {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("bin"), 0o755)) //nolint:gosec
	}
	return dir
}

func TestValidateBinaryMatrixRequiresLinuxAmd64(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{"darwin-arm64": "bin/b"}
	dir := writeTestPack(t, binaries)
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, platformLinuxAmd64)
	require.ErrorContains(t, err, "'binary'")
}

func TestValidateBinaryMatrixRequiresHostPlatform(t *testing.T) {
	t.Parallel()

	if workspace.CurrentPlatform() == platformLinuxAmd64 {
		t.Skip("host platform is linux-amd64; the mandatory-platform check subsumes this")
	}
	binaries := map[string]string{platformLinuxAmd64: "bin/b"}
	dir := writeTestPack(t, binaries)
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, workspace.CurrentPlatform())
	require.ErrorContains(t, err, "'binary'")
}

func TestValidateBinaryMatrixMissingFile(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{
		platformLinuxAmd64:          "bin/b",
		workspace.CurrentPlatform(): "bin/host",
	}
	dir := writeTestPack(t, binaries)
	require.NoError(t, os.Remove(filepath.Join(dir, "bin", "b")))
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, "bin/b")
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

	rel := "bin/pulumi-analyzer-mypack-linux-amd64"
	dir := writeTestPack(t, map[string]string{"linux-amd64": rel})

	tgz, err := buildBinaryArtifact(dir, rel, "mypack", "linux-amd64", testRuntime)
	require.NoError(t, err)

	entries := tarEntries(t, tgz)
	require.Len(t, entries, 2)
	assert.Contains(t, entries, "package/pulumi-analyzer-mypack")

	// The generated manifest must declare the canonical binary for the platform so
	// consumers dispatch to it without a language toolchain.
	manifest := string(entries["package/PulumiPolicy.yaml"])
	assert.Contains(t, manifest, "binary:")
	assert.Contains(t, manifest, "linux-amd64: pulumi-analyzer-mypack")
}

func TestBuildBinaryArtifactWindowsSuffix(t *testing.T) {
	t.Parallel()

	rel := "bin/pulumi-analyzer-mypack-windows-amd64.exe"
	dir := writeTestPack(t, map[string]string{"windows-amd64": rel})

	tgz, err := buildBinaryArtifact(dir, rel, "mypack", "windows-amd64", testRuntime)
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

	binaries := map[string]string{
		"linux-amd64":   "bin/pulumi-analyzer-mypack-linux-amd64",
		"windows-amd64": "bin/pulumi-analyzer-mypack-windows-amd64.exe",
		"darwin-arm64":  "bin/pulumi-analyzer-mypack-darwin-arm64",
	}
	tgz := buildTGZ(t, map[string]string{
		"package/PulumiPolicy.yaml":                            "runtime: nodejs\n",
		"package/bin/pulumi-analyzer-mypack-linux-amd64":       "bin",
		"package/bin/pulumi-analyzer-mypack-windows-amd64.exe": "bin",
		"package/index.js":                                     "// not a binary",
	})

	found := sourceTarballBinaries(tgz, binaries)
	assert.Equal(t, []string{
		"bin/pulumi-analyzer-mypack-linux-amd64",
		"bin/pulumi-analyzer-mypack-windows-amd64.exe",
	}, found)
}

func TestSourceTarballBinariesNoneFound(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{"linux-amd64": "bin/pulumi-analyzer-mypack-linux-amd64"}
	tgz := buildTGZ(t, map[string]string{
		"package/PulumiPolicy.yaml": "runtime: nodejs\n",
		"package/index.js":          "// not a binary",
	})

	assert.Empty(t, sourceTarballBinaries(tgz, binaries))
}

func TestPackLocationSelection(t *testing.T) {
	t.Parallel()

	platform := workspace.CurrentPlatform()

	t.Run("binary for this platform", func(t *testing.T) {
		t.Parallel()
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "p",
			PackLocation:  "source-key",
			PackLocations: map[string]string{platform: "bin-key"},
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "bin-key", loc)
	})

	t.Run("platform gap falls back to source", func(t *testing.T) {
		t.Parallel()
		other := "linux-amd64"
		if platform == other {
			other = "darwin-arm64"
		}
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "p",
			PackLocation:  "source-key",
			PackLocations: map[string]string{other: "bin-key"},
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "source-key", loc)
	})

	t.Run("legacy pack", func(t *testing.T) {
		t.Parallel()
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:         "p",
			PackLocation: "source-key",
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "source-key", loc)
	})

	t.Run("binary only, platform missing", func(t *testing.T) {
		t.Parallel()
		other := "linux-amd64"
		if platform == other {
			other = "darwin-arm64"
		}
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "p",
			PackLocations: map[string]string{other: "bin-key"},
		}}
		_, err := rp.packLocation()
		require.ErrorContains(t, err, platform)
		require.ErrorContains(t, err, other)
	})
}

func TestInstallRequiredPolicySkipsDependenciesForBinaryPack(t *testing.T) {
	t.Parallel()

	rel := "bin/pulumi-analyzer-mypack-" + workspace.CurrentPlatform()
	packDir := writeTestPack(t, map[string]string{workspace.CurrentPlatform(): rel})
	tgz, err := buildBinaryArtifact(packDir, rel, "mypack", workspace.CurrentPlatform(), testRuntime)
	require.NoError(t, err)

	finalDir := filepath.Join(t.TempDir(), "installed")
	// ctx is never used past the binary short-circuit: a nil-host context proves no
	// language runtime was resolved.
	err = installRequiredPolicy(nil, finalDir, io.NopCloser(bytes.NewReader(tgz)), io.Discard, io.Discard)
	require.NoError(t, err)

	proj, err := workspace.LoadPolicyPack(filepath.Join(finalDir, "PulumiPolicy.yaml"))
	require.NoError(t, err)
	binName := canonicalBinaryName("mypack", workspace.CurrentPlatform())
	require.Equal(t, map[string]string{workspace.CurrentPlatform(): binName}, proj.Binary)

	info, err := os.Stat(filepath.Join(finalDir, binName))
	require.NoError(t, err)
	if goruntime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o111)
	}
}
