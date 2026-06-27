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

package oci

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// fakePod is a minimal PodManager for unit tests. Only ImageExists is meaningful;
// the rest panic, because the paths under test never reach them.
type fakePod struct{ imageExists bool }

func (f fakePod) CreateNetwork(context.Context) (Network, error)                   { panic("unused") }
func (f fakePod) RunContainer(context.Context, ContainerConfig) (Container, error) { panic("unused") }
func (f fakePod) WaitContainer(context.Context, Container) (int, error)            { panic("unused") }

func (f fakePod) ContainerLogs(context.Context, Container, bool) (io.ReadCloser, error) {
	panic("unused")
}
func (f fakePod) StopContainer(context.Context, Container) error       { panic("unused") }
func (f fakePod) CreateVolume(context.Context, string) (Volume, error) { panic("unused") }

func (f fakePod) RunToCompletion(context.Context, ContainerConfig, io.Writer) (string, error) {
	panic("unused")
}

func (f fakePod) CopyFromImage(context.Context, string, string, Volume, string) error {
	panic("unused")
}
func (f fakePod) ImageExists(context.Context, string) (bool, error) { return f.imageExists, nil }
func (f fakePod) PullImage(context.Context, string) error           { panic("unused") }
func (f fakePod) Cleanup(context.Context) error                     { panic("unused") }

// When a provider image is absent and no registry is configured to install it,
// ensureImage must bail out with an actionable error rather than letting the
// downstream docker run/copy fail cryptically.
func TestEnsureImageBailsActionablyWhenAbsentAndNoRegistry(t *testing.T) {
	t.Parallel()
	h := &containerHost{pod: fakePod{imageExists: false}}
	err := h.ensureImage(context.Background(), "random", "pulumi-provider-random:v4.21.0")
	require.Error(t, err)
	// Names the provider and the missing ref, and points at both fixes.
	require.Contains(t, err.Error(), `provider "random"`)
	require.Contains(t, err.Error(), "pulumi-provider-random:v4.21.0")
	require.Contains(t, err.Error(), "pulumi install")
	require.Contains(t, err.Error(), "PULUMI_POD_PLUGIN_REGISTRY")
}

// When the image is already present, ensureImage is a no-op regardless of registry.
func TestEnsureImageNoopWhenPresent(t *testing.T) {
	t.Parallel()
	h := &containerHost{pod: fakePod{imageExists: true}}
	require.NoError(t, h.ensureImage(context.Background(), "random", "anything:v1"))
}

// A dynamic provider runs from the program image with the SDK's own entrypoint,
// selected via the role env, and with nothing injected: no provider image is
// resolved, no ensure/pull, no volume. fakePod's methods all panic, so a green
// run also asserts the pod is never touched on the dynamic path.
func TestProviderContainerDynamicRunsFromProgramImage(t *testing.T) {
	t.Parallel()
	h := &containerHost{
		pod:          fakePod{},
		engineHost:   "engine",
		programImage: "my-program:v1",
	}
	cfg, err := h.providerContainer(context.Background(), workspace.PluginDescriptor{Name: "pulumi-nodejs"})
	require.NoError(t, err)
	require.Equal(t, "my-program:v1", cfg.Image)
	require.Equal(t, "container:engine", cfg.Network)
	require.Equal(t, roleDynamicProvider, cfg.Env[roleEnvVar])
	require.Empty(t, cfg.Volumes, "dynamic providers inject nothing")
	require.Empty(t, cfg.Entrypoint, "the image's bootstrap shim selects the entrypoint via the role env")
}

// Without a program image there is nothing to run the dynamic provider from, so it
// fails with an actionable message rather than a cryptic downstream docker error.
func TestProviderContainerDynamicRequiresProgramImage(t *testing.T) {
	t.Parallel()
	h := &containerHost{pod: fakePod{}, engineHost: "engine"}
	_, err := h.providerContainer(context.Background(), workspace.PluginDescriptor{Name: "pulumi-python"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PULUMI_POD_PROGRAM_IMAGE")
}

// projectedProviderEnv copies the engine env onto a provider container (spawn
// parity) so credentials travel as environment — but drops vars owned by the
// engine's own container context so they don't clobber the provider image's.
// Not parallel: it mutates the process environment via t.Setenv.
func TestProjectedProviderEnv(t *testing.T) {
	t.Setenv("OCI_TEST_FAKE_CRED", "sekret-value")
	t.Setenv("DOCKER_HOST", "unix:///host/only/docker.sock")
	t.Setenv("PULUMI_POD_ID", "pod-xyz")
	t.Setenv("PULUMI_HOME", "/workspace/.pulumi-pod")
	e := projectedProviderEnv()
	require.Equal(t, "sekret-value", e["OCI_TEST_FAKE_CRED"], "credentials project through")
	require.NotContains(t, e, "DOCKER_HOST", "host-context docker socket var is dropped")
	require.NotContains(t, e, "PULUMI_POD_ID", "pod-control vars are dropped")
	require.NotContains(t, e, "PULUMI_HOME", "engine-orchestration path (pod home) is dropped")
	require.NotContains(t, e, "PATH", "the provider image owns its PATH")
}
