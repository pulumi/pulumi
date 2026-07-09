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
	"testing"

	"github.com/stretchr/testify/assert"
)

func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func fakeFiles(fs ...string) func(string) bool {
	set := map[string]bool{}
	for _, f := range fs {
		set[f] = true
	}
	return func(p string) bool { return set[p] }
}

func TestDetectModeOnHost(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeHost, detectMode(fakeEnv(nil), fakeFiles()))
}

func TestDetectModeInContainerWithSocket(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeSibling, detectMode(fakeEnv(nil), fakeFiles("/.dockerenv", "/var/run/docker.sock")))
	assert.Equal(t, ModeSibling, detectMode(fakeEnv(nil), fakeFiles("/run/.containerenv", "/var/run/docker.sock")))
}

func TestDetectModeInContainerWithDockerHost(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeSibling,
		detectMode(fakeEnv(map[string]string{"DOCKER_HOST": "unix:///x.sock"}), fakeFiles("/.dockerenv")))
}

func TestDetectModeInContainerNoSocket(t *testing.T) {
	t.Parallel()
	// In a container with no socket we still return ModeHost; the runtime
	// probe will then fail loudly (no docker CLI / no daemon).
	assert.Equal(t, ModeHost, detectMode(fakeEnv(nil), fakeFiles("/.dockerenv")))
}

func TestDetectModeEnvOverride(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeSibling, detectMode(fakeEnv(map[string]string{EnvNetworkMode: "sibling"}), fakeFiles()))
	assert.Equal(t, ModeHost,
		detectMode(fakeEnv(map[string]string{EnvNetworkMode: "host"}), fakeFiles("/.dockerenv", "/var/run/docker.sock")))
}
