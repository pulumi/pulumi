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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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

// recordingPod is a fakePod that records the containers StopContainer was called on, so a
// test can assert which containers ReleaseContext reaped.
type recordingPod struct {
	fakePod
	stopped *[]Container
}

func (r recordingPod) StopContainer(_ context.Context, c Container) error {
	*r.stopped = append(*r.stopped, c)
	return nil
}

func containerIDs(cs []Container) []string {
	ids := make([]string, len(cs))
	for i, c := range cs {
		ids[i] = c.ID
	}
	return ids
}

func trackedIDs(ps []podPlugin) []string {
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.container.ID
	}
	return ids
}

// noSignal is a no-op cancellation function for tracking containers in tests that don't
// exercise SignalCancellation.
func noSignal(context.Context) error { return nil }

// When a provider image is absent and no registry is configured to install it,
// ensureImage must bail out with an actionable error rather than letting the
// downstream docker run/copy fail cryptically.
func TestEnsureImageBailsActionablyWhenAbsentAndNoRegistry(t *testing.T) {
	t.Parallel()
	h := &containerHost{pod: fakePod{imageExists: false}}
	err := h.ensureImage(t.Context(), "random", "pulumi-provider-random:v4.21.0")
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
	require.NoError(t, h.ensureImage(t.Context(), "random", "anything:v1"))
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
	cfg, err := h.providerContainer(t.Context(), workspace.PluginDescriptor{Name: "pulumi-nodejs"})
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
	_, err := h.providerContainer(t.Context(), workspace.PluginDescriptor{Name: "pulumi-python"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PULUMI_POD_PROGRAM_IMAGE")
}

// uniqueContainerName appends a process-unique suffix, so two calls with the same base
// never produce the same name — the property that keeps provider/policy containers from
// colliding across a `pulumi up`'s preview and apply phases.
func TestUniqueContainerName(t *testing.T) {
	t.Parallel()
	first := uniqueContainerName("provider-aws")
	second := uniqueContainerName("provider-aws")
	require.NotEqual(t, first, second)
	require.Regexp(t, `^provider-aws-\d+$`, first)
	require.Regexp(t, `^provider-aws-\d+$`, second)
}

// The engine boots a provider once per deployment phase (preview, then apply) from a fresh
// host, and a program may use two instances of one provider package; both map to the same
// descriptor. Each started container must still get a distinct name, or the second start
// collides with the first (which lives until the pod is torn down). The dynamic-provider
// path returns early without touching the pod, so fakePod's panicking methods are unused.
func TestProviderContainerNamesAreUnique(t *testing.T) {
	t.Parallel()
	h := &containerHost{pod: fakePod{}, engineHost: "engine", programImage: "my-program:v1"}
	desc := workspace.PluginDescriptor{Name: "pulumi-nodejs"}
	first, err := h.providerContainer(t.Context(), desc)
	require.NoError(t, err)
	second, err := h.providerContainer(t.Context(), desc)
	require.NoError(t, err)
	require.NotEqual(t, first.Name, second.Name,
		"two starts of the same provider must get distinct container names")
	require.Regexp(t, `^provider-pulumi-nodejs-\d+$`, first.Name)
}

// ReleaseContext stops exactly the containers tracked for the released context — the way
// the engine reaps a deployment phase's providers when it releases that phase's context —
// and leaves other contexts' containers running. It also delegates to the base host. A
// zero-value plugin.Context is its own LifetimeContext, so it serves as a distinct key.
func TestReleaseContextReapsTrackedContainers(t *testing.T) {
	t.Parallel()
	var stopped []Container
	h := &containerHost{
		Host:    &plugin.MockHost{},
		pod:     recordingPod{stopped: &stopped},
		started: map[*plugin.Context][]podPlugin{},
	}
	ctxA, ctxB := &plugin.Context{}, &plugin.Context{}
	h.trackPlugin(ctxA, Container{ID: "a1", Name: "provider-aws-1"}, noSignal)
	h.trackPlugin(ctxA, Container{ID: "a2", Name: "policy-pack-2"}, noSignal)
	h.trackPlugin(ctxB, Container{ID: "b1", Name: "provider-gcp-3"}, noSignal)

	// Map lookups go by pointer identity; assert on those and on the StopContainer record
	// rather than testify's Contains, which compares keys with reflect.DeepEqual (two
	// zero-value contexts would compare equal).
	require.NoError(t, h.ReleaseContext(ctxA))
	require.ElementsMatch(t, []string{"a1", "a2"}, containerIDs(stopped),
		"only the released context's containers are stopped")
	require.Empty(t, h.started[ctxA], "the released context is forgotten")
	require.Equal(t, []string{"b1"}, trackedIDs(h.started[ctxB]),
		"an unreleased context's containers stay tracked")
	require.Len(t, h.started, 1)

	stopped = nil
	require.NoError(t, h.ReleaseContext(ctxB))
	require.ElementsMatch(t, []string{"b1"}, containerIDs(stopped))
	require.Empty(t, h.started)
}

// SignalCancellation forwards a graceful cancel to every tracked plugin across all contexts —
// the providers and policy packs the container host attached directly, which the base host's
// own SignalCancellation never sees. Without this, a Ctrl-C during an OCI deployment would not
// reach in-flight provider operations.
func TestSignalCancellationForwardsToTrackedPlugins(t *testing.T) {
	t.Parallel()
	var signaled []string
	h := &containerHost{
		Host:    &plugin.MockHost{},
		started: map[*plugin.Context][]podPlugin{},
	}
	rec := func(name string) func(context.Context) error {
		return func(context.Context) error { signaled = append(signaled, name); return nil }
	}
	ctxA, ctxB := &plugin.Context{}, &plugin.Context{}
	h.trackPlugin(ctxA, Container{Name: "provider-aws-1"}, rec("aws"))
	h.trackPlugin(ctxB, Container{Name: "policy-pack-2"}, rec("policy"))

	require.NoError(t, h.SignalCancellation())
	require.ElementsMatch(t, []string{"aws", "policy"}, signaled,
		"a graceful cancel reaches every tracked plugin across all contexts")
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

// writePolicyPack writes a PulumiPolicy.yaml into a fresh dir and returns the dir.
func writePolicyPack(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PulumiPolicy.yaml"), []byte(yaml), 0o600))
	return dir
}

// A policy pack declaring runtime: oci resolves to its declared image, so the
// container host runs it as a container rather than spawning it.
func TestPolicyPackImageOCI(t *testing.T) {
	t.Parallel()
	dir := writePolicyPack(t, "runtime:\n  name: oci\n  options:\n    image: my-policy:latest\n")
	image, ok, err := policyPackImage(dir)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "my-policy:latest", image)
}

// The normal form: the engine receives an already-resolved image ref, not a
// directory. policyPackImage treats anything that is not a local directory as the
// image itself, so the engine runs the ref and reads no manifest off a mount — the
// manifest-projection hack removed.
func TestPolicyPackImageRef(t *testing.T) {
	t.Parallel()
	// A ref that does not exist as a directory on disk is taken verbatim.
	image, ok, err := policyPackImage("oci-smoke-policy:latest")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "oci-smoke-policy:latest", image)

	// A registry-qualified ref is likewise passed through untouched.
	image, ok, err = policyPackImage("ghcr.io/acme/policy:v1.2.3")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ghcr.io/acme/policy:v1.2.3", image)
}

// A policy pack on a normal language runtime is not an OCI pack: ok is false, so
// the container host falls back to the base host's spawn path.
func TestPolicyPackImageNonOCIFallsBack(t *testing.T) {
	t.Parallel()
	dir := writePolicyPack(t, "runtime: nodejs\n")
	image, ok, err := policyPackImage(dir)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, image)
}

// An OCI policy pack that names no image is an error — there is nothing to run.
func TestPolicyPackImageOCIRequiresImage(t *testing.T) {
	t.Parallel()
	dir := writePolicyPack(t, "runtime:\n  name: oci\n")
	_, ok, err := policyPackImage(dir)
	require.Error(t, err)
	require.False(t, ok)
	require.Contains(t, err.Error(), "no image option")
}

// Container names derive from a policy pack's filesystem path, which is not a legal
// Docker name; sanitizeContainerName maps the illegal characters to dashes.
func TestSanitizeContainerName(t *testing.T) {
	t.Parallel()
	require.Equal(t, "policy", sanitizeContainerName("policy"))
	require.Equal(t, "my-pack_v1.2", sanitizeContainerName("my-pack_v1.2"))
	require.Equal(t, "a-b-c", sanitizeContainerName("a/b:c"))
}
