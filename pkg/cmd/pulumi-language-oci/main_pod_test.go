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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/oci"
	"github.com/stretchr/testify/assert"
)

// TestPodAdvertiseHost pins the program→engine address wiring across runtimes: on CRI the
// program shares the engine's sandbox netns, so it dials the engine on loopback regardless of
// any advertised DNS name; on the docker/nerdctl bridge it dials the advertised container name.
func TestPodAdvertiseHost(t *testing.T) {
	t.Run("cri uses loopback even with an advertise host set", func(t *testing.T) {
		t.Setenv(oci.PodRuntimeEnvVar, "cri")
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "engine-container")
		assert.Equal(t, "127.0.0.1", podAdvertiseHost())
	})

	t.Run("bridge uses the advertised container name", func(t *testing.T) {
		t.Setenv(oci.PodRuntimeEnvVar, "") // docker
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "engine-container")
		assert.Equal(t, "engine-container", podAdvertiseHost())
	})

	t.Run("bridge falls back to this container's hostname", func(t *testing.T) {
		t.Setenv(oci.PodRuntimeEnvVar, "")
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "")
		host, _ := os.Hostname()
		assert.Equal(t, host, podAdvertiseHost())
	})
}

// TestRewriteHost covers the host-portion rewrite the pod-mode Run path applies to the monitor
// and engine addresses.
func TestRewriteHost(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "engine:50051", rewriteHost("127.0.0.1:50051", "engine"),
		"the DNS-name rewrite the docker bridge needs")
	assert.Equal(t, "127.0.0.1:50051", rewriteHost("0.0.0.0:50051", "127.0.0.1"),
		"the loopback rewrite CRI needs (engine binds 0.0.0.0)")
	assert.Equal(t, "not-a-host-port", rewriteHost("not-a-host-port", "engine"),
		"a non host:port value is returned unchanged")
}
