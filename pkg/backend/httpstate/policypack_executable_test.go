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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackLocationPlatformSelection(t *testing.T) {
	t.Parallel()

	t.Run("single artifact location", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name: "pack", PackLocation: "https://single-artifact",
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "https://single-artifact", loc)
	})

	t.Run("platform map picks host platform", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name: "pack",
			PackLocations: map[string]string{
				workspace.CurrentPlatform(): "https://mine",
				"made-up-platform":          "https://other",
			},
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "https://mine", loc)
	})

	t.Run("host platform missing is a loud error", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "pack",
			PackLocations: map[string]string{"made-up-platform": "https://other"},
		}}
		_, err := rp.packLocation()
		require.Error(t, err)
		assert.ErrorContains(t, err, "pack")
		assert.ErrorContains(t, err, workspace.CurrentPlatform())
		assert.ErrorContains(t, err, "made-up-platform")
	})
}

func writeExecutablePack(t *testing.T, binaries map[string]string) string {
	packDir := t.TempDir()
	var sb strings.Builder
	sb.WriteString("runtime:\n  name: executable\n  options:\n    binaries:\n")
	for platform, rel := range binaries {
		fmt.Fprintf(&sb, "      %s: %s\n", platform, filepath.ToSlash(rel))
	}
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(sb.String()), 0o600))
	for _, rel := range binaries {
		path := filepath.Join(packDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		//nolint:gosec // G306: File needs to be executable (0755)
		require.NoError(t, os.WriteFile(path, []byte("binary"), 0o755))
	}
	return packDir
}

func TestValidateExecutableMatrix(t *testing.T) {
	t.Parallel()

	// The matrix is validated without reference to the host platform, so these cases name
	// platforms outright. Deriving one of them from workspace.CurrentPlatform() would collide
	// with linux-amd64 on a linux-amd64 machine and silently collapse the map.
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		binaries := map[string]string{
			"linux-amd64":  filepath.Join("bin", "linux"),
			"darwin-arm64": filepath.Join("bin", "darwin"),
		}
		packDir := writeExecutablePack(t, binaries)
		require.NoError(t, validateExecutableMatrix(packDir, binaries))
	})

	t.Run("missing linux-amd64", func(t *testing.T) {
		t.Parallel()
		binaries := map[string]string{"darwin-arm64": filepath.Join("bin", "darwin")}
		packDir := writeExecutablePack(t, binaries)
		err := validateExecutableMatrix(packDir, binaries)
		assert.ErrorContains(t, err, "linux-amd64")
	})

	t.Run("declared binary missing on disk", func(t *testing.T) {
		t.Parallel()
		binaries := map[string]string{
			"linux-amd64":  filepath.Join("bin", "linux"),
			"darwin-arm64": filepath.Join("bin", "darwin"),
		}
		packDir := writeExecutablePack(t, binaries)
		require.NoError(t, os.Remove(filepath.Join(packDir, "bin", "linux")))
		err := validateExecutableMatrix(packDir, binaries)
		assert.ErrorContains(t, err, "linux-amd64")
		assert.ErrorContains(t, err, filepath.Join("bin", "linux"))
	})

	// The publishing host's platform need not be declared. Publish still needs a host binary to
	// read the pack's metadata, but booting the analyzer is what reports that.
	t.Run("linux-amd64 alone is a valid matrix", func(t *testing.T) {
		t.Parallel()
		binaries := map[string]string{workspace.PlatformLinuxAmd64: filepath.Join("bin", "linux")}
		packDir := writeExecutablePack(t, binaries)
		require.NoError(t, validateExecutableMatrix(packDir, binaries))
	})
}

func TestBuildExecutablePlatformTarball(t *testing.T) {
	t.Parallel()

	binRel := filepath.Join("bin", "tool")
	packDir := writeExecutablePack(t, map[string]string{"linux-amd64": binRel})
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "index.ts"), []byte("// source"), 0o600))

	tgz, err := buildExecutablePlatformTarball(packDir, binRel)
	require.NoError(t, err)

	extractDir := t.TempDir()
	require.NoError(t, archive.ExtractTGZ(io.NopCloser(bytes.NewReader(tgz)), extractDir))

	assert.FileExists(t, filepath.Join(extractDir, "package", "PulumiPolicy.yaml"))
	info, err := os.Stat(filepath.Join(extractDir, "package", binRel))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o111, "binary in artifact must keep the executable bit")
	}
	assert.NoFileExists(t, filepath.Join(extractDir, "package", "index.ts"),
		"per-platform artifacts must contain only the manifest and one binary")
}
