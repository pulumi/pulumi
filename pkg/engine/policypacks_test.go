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

package engine

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	ociruntime "github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"
)

type fakeOCIRequiredPolicy struct {
	imageRef string
}

func (p *fakeOCIRequiredPolicy) Name() string               { return "fake-oci" }
func (p *fakeOCIRequiredPolicy) Version() string            { return "1.0.0" }
func (p *fakeOCIRequiredPolicy) ImageRef() string           { return p.imageRef }
func (p *fakeOCIRequiredPolicy) Installed() bool            { return false }
func (p *fakeOCIRequiredPolicy) LocalPath() (string, error) { return "", nil }
func (p *fakeOCIRequiredPolicy) Download(
	ctx context.Context, wrapper func(io.ReadCloser, int64) io.ReadCloser,
) (io.ReadCloser, int64, error) {
	panic("Download must not be called for OCI policy packs")
}

func (p *fakeOCIRequiredPolicy) Install(ctx *plugin.Context, content io.ReadCloser, stdout, stderr io.Writer) error {
	panic("Install must not be called for OCI policy packs")
}
func (p *fakeOCIRequiredPolicy) Config() map[string]*json.RawMessage { return nil }
func (p *fakeOCIRequiredPolicy) ResolveEnvironments(ctx context.Context) (*ResolvedPolicyEnvironment, error) {
	return nil, nil
}

func TestInstallPolicyPackPullsOCIImage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	// Stub docker on PATH that records the pull.
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	dockerPath := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(dockerPath, []byte(
		"#!/bin/sh\necho \"$@\" >> "+record+"\n"), 0o600))
	require.NoError(t, os.Chmod(dockerPath, 0o700))
	t.Setenv("PATH", dir)
	t.Setenv(ociruntime.EnvContainerRuntime, "")

	policy := &fakeOCIRequiredPolicy{imageRef: "ghcr.io/acme/pack@sha256:abc"}
	err := installPolicyPack(t.Context(), nil, nil, policy)
	require.NoError(t, err)

	b, err := os.ReadFile(record)
	require.NoError(t, err)
	assert.Contains(t, string(b), "pull ghcr.io/acme/pack@sha256:abc")
}

func TestInstallPolicyPackOCIRuntimeMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // nothing on PATH
	t.Setenv(ociruntime.EnvContainerRuntime, "")
	policy := &fakeOCIRequiredPolicy{imageRef: "ghcr.io/acme/pack@sha256:abc"}
	err := installPolicyPack(t.Context(), nil, nil, policy)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container runtime")
	assert.Contains(t, err.Error(), "fake-oci")
}
