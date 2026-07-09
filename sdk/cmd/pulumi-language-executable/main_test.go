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
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
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

// The executable runtime is not a program runtime; Run must refuse rather than silently doing nothing.
func TestRunIsRefused(t *testing.T) {
	t.Parallel()

	_, err := (&executableLanguageHost{}).Run(t.Context(), &pulumirpc.RunRequest{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot run Pulumi programs")
}

// AboutResponse.Executable is an absolute path to a language runtime binary. This host has none,
// so it must report nothing rather than something path-shaped that isn't a path.
func TestAboutReportsNoRuntime(t *testing.T) {
	t.Parallel()

	about, err := (&executableLanguageHost{}).About(t.Context(), &pulumirpc.AboutRequest{})
	require.NoError(t, err)
	assert.Empty(t, about.Executable)
	assert.Empty(t, about.Version)
}

type recordingRunPluginServer struct {
	grpc.ServerStream
	ctx       context.Context
	responses []*pulumirpc.RunPluginResponse
}

func (s *recordingRunPluginServer) Context() context.Context { return s.ctx }

func (s *recordingRunPluginServer) Send(resp *pulumirpc.RunPluginResponse) error {
	s.responses = append(s.responses, resp)
	return nil
}

func (s *recordingRunPluginServer) exitCode(t *testing.T) int32 {
	t.Helper()
	for _, resp := range s.responses {
		if code, ok := resp.Output.(*pulumirpc.RunPluginResponse_Exitcode); ok {
			return code.Exitcode
		}
	}
	t.Fatal("no exit code was sent to the engine")
	return 0
}

func writeExecutablePackBinary(t *testing.T, script string) (packDir, binRel string) {
	t.Helper()
	packDir = t.TempDir()
	binRel = filepath.Join("bin", "policy")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, binRel), []byte(script), 0o755))
	return packDir, binRel
}

func TestRunPluginPropagatesExitCode(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("the test pack binary is a shell script")
	}
	t.Parallel()

	packDir, binRel := writeExecutablePackBinary(t, "#!/bin/sh\nexit 3\n")
	server := &recordingRunPluginServer{ctx: t.Context()}

	err := (&executableLanguageHost{}).RunPlugin(&pulumirpc.RunPluginRequest{
		Info: programInfo(t, packDir, map[string]string{workspace.CurrentPlatform(): binRel}),
		Pwd:  packDir,
	}, server)

	require.NoError(t, err)
	assert.Equal(t, int32(3), server.exitCode(t))
}

func TestRunPluginMissingPlatformIsLoud(t *testing.T) {
	t.Parallel()

	other := "linux-amd64"
	if workspace.CurrentPlatform() == other {
		other = "darwin-arm64"
	}
	server := &recordingRunPluginServer{ctx: t.Context()}

	err := (&executableLanguageHost{}).RunPlugin(&pulumirpc.RunPluginRequest{
		Info: programInfo(t, t.TempDir(), map[string]string{other: filepath.Join("bin", "policy")}),
	}, server)

	require.Error(t, err)
	assert.ErrorContains(t, err, "does not provide a binary for "+workspace.CurrentPlatform())
	assert.Empty(t, server.responses)
}
