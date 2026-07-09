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
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func programInfo(t *testing.T, dir string, binaries map[string]string) *pulumirpc.ProgramInfo {
	t.Helper()
	entries := map[string]any{}
	for platform, rel := range binaries {
		entries[platform] = rel
	}
	options, err := structpb.NewStruct(map[string]any{"binaries": entries})
	require.NoError(t, err)
	return &pulumirpc.ProgramInfo{
		RootDirectory:    dir,
		ProgramDirectory: dir,
		EntryPoint:       ".",
		Options:          options,
	}
}

func TestHostBinarySelectsCurrentPlatform(t *testing.T) {
	t.Parallel()

	other := "linux-amd64"
	if workspace.CurrentPlatform() == other {
		other = "darwin-arm64"
	}
	info := programInfo(t, t.TempDir(), map[string]string{
		workspace.CurrentPlatform(): "bin/policy",
		other:                       "bin/policy-other",
	})

	binary, err := hostBinary(info)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean("bin/policy"), binary)
}

func TestHostBinaryMissingPlatformIsLoud(t *testing.T) {
	t.Parallel()

	other := "linux-amd64"
	if workspace.CurrentPlatform() == other {
		other = "darwin-arm64"
	}
	info := programInfo(t, t.TempDir(), map[string]string{other: "bin/policy-other"})

	_, err := hostBinary(info)
	require.Error(t, err)
	assert.ErrorContains(t, err, "does not provide a binary for "+workspace.CurrentPlatform())
	assert.ErrorContains(t, err, other)
}

func TestHostBinaryRejectsEscapingPath(t *testing.T) {
	t.Parallel()

	info := programInfo(t, t.TempDir(), map[string]string{workspace.CurrentPlatform(): "../../etc/passwd"})

	_, err := hostBinary(info)
	require.Error(t, err)
	assert.ErrorContains(t, err, "must not escape the policy pack directory")
}

func TestEnsureExecutableMarksBinaryExecutable(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("file mode bits are not meaningful on Windows")
	}
	t.Parallel()

	packDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "bin"), 0o755))
	binary := filepath.Join(packDir, "bin", "policy")
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0o600))

	info := programInfo(t, packDir, map[string]string{workspace.CurrentPlatform(): "bin/policy"})
	require.NoError(t, ensureExecutable(info))

	stat, err := os.Stat(binary)
	require.NoError(t, err)
	assert.NotZero(t, stat.Mode()&0o111, "binary must be executable after install")
}

func TestEnsureExecutableMissingBinaryIsLoud(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("ensureExecutable is a no-op on Windows")
	}
	t.Parallel()

	info := programInfo(t, t.TempDir(), map[string]string{workspace.CurrentPlatform(): "bin/policy"})

	err := ensureExecutable(info)
	require.Error(t, err)
	assert.ErrorContains(t, err, "policy pack binary not found")
}

// The executable runtime is not a program runtime; Run must refuse rather than silently doing nothing.
func TestRunIsRefused(t *testing.T) {
	t.Parallel()

	_, err := (&executableLanguageHost{}).Run(t.Context(), &pulumirpc.RunRequest{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot run Pulumi programs")
}
