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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeLookPath(available map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if p, ok := available[name]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
}

func TestDetectRuntimeProbeOrder(t *testing.T) {
	t.Parallel()
	rt, err := DetectRuntime(fakeLookPath(map[string]string{
		"podman": "/usr/bin/podman",
		"docker": "/usr/bin/docker",
	}))
	require.NoError(t, err)
	assert.Equal(t, "docker", rt.Name)
	assert.Equal(t, "/usr/bin/docker", rt.Path)
}

func TestDetectRuntimeFallsBack(t *testing.T) {
	t.Parallel()
	rt, err := DetectRuntime(fakeLookPath(map[string]string{"nerdctl": "/usr/bin/nerdctl"}))
	require.NoError(t, err)
	assert.Equal(t, "nerdctl", rt.Name)
}

func TestDetectRuntimeNoneFound(t *testing.T) {
	t.Parallel()
	_, err := DetectRuntime(fakeLookPath(nil))
	require.ErrorIs(t, err, ErrNoContainerRuntime)
}

func TestDetectRuntimeEnvOverride(t *testing.T) {
	t.Setenv(EnvContainerRuntime, "podman")
	rt, err := DetectRuntime(fakeLookPath(map[string]string{
		"podman": "/opt/podman",
		"docker": "/usr/bin/docker",
	}))
	require.NoError(t, err)
	assert.Equal(t, "podman", rt.Name)
	assert.Equal(t, "/opt/podman", rt.Path)
}

func TestDetectRuntimeEnvOverrideMissing(t *testing.T) {
	t.Setenv(EnvContainerRuntime, "podman")
	_, err := DetectRuntime(fakeLookPath(map[string]string{"docker": "/usr/bin/docker"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "podman")
	assert.Contains(t, err.Error(), EnvContainerRuntime)
}
