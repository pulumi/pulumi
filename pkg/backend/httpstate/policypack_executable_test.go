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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackLocationPlatformSelection(t *testing.T) {
	t.Parallel()

	t.Run("legacy single location", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name: "pack", PackLocation: "https://legacy",
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "https://legacy", loc)
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

func TestInstallRequiredPolicyExecutable(t *testing.T) {
	t.Parallel()

	packDir := t.TempDir()
	binRel := filepath.Join("bin", "policy")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, binRel), []byte("#!/bin/sh\nexit 0\n"), 0o644))
	manifest := fmt.Sprintf(
		"runtime:\n  name: executable\n  options:\n    binaries:\n      %s: bin/policy\n",
		workspace.CurrentPlatform())
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(manifest), 0o600))

	tgz, err := archive.TGZ(packDir, "package", false)
	require.NoError(t, err)

	finalDir := filepath.Join(t.TempDir(), "pulumi-analyzer-pack-v1")
	// The plugin context is never used on the executable path; a zero value
	// guarantees the language-runtime/dependency-install path is not reached.
	err = installRequiredPolicy(&plugin.Context{}, finalDir,
		io.NopCloser(bytes.NewReader(tgz)), io.Discard, io.Discard)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(finalDir, "PulumiPolicy.yaml"))
	info, err := os.Stat(filepath.Join(finalDir, binRel))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o111, "installed binary must be executable")
	}
}
