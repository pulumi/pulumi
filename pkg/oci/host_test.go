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
