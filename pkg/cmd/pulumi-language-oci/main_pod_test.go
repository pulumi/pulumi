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

	"github.com/stretchr/testify/assert"
)

// TestPodAdvertiseHost pins the program→engine address wiring. The runtime decision does NOT
// live here: the wrapper knows what is reachable and supplies it, setting 127.0.0.1 on CRI
// (all sandbox members share one netns) and the engine's container name on the docker/nerdctl
// bridge. The language host only reads that back, which is what keeps it runtime-agnostic.
func TestPodAdvertiseHost(t *testing.T) {
	t.Run("uses the host the wrapper supplied", func(t *testing.T) {
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "engine-container")
		assert.Equal(t, "engine-container", podAdvertiseHost())
	})

	t.Run("honors a loopback host for shared-netns runtimes", func(t *testing.T) {
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "127.0.0.1")
		assert.Equal(t, "127.0.0.1", podAdvertiseHost())
	})

	t.Run("falls back to this container's hostname", func(t *testing.T) {
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "")
		host, _ := os.Hostname()
		assert.Equal(t, host, podAdvertiseHost())
	})
}

// fixedNamer is a containerNamer that namespaces like the docker pod manager.
type fixedNamer struct{ prefix string }

func (f fixedNamer) ContainerName(logical string) string { return f.prefix + logical }

// TestProgramAdvertiseHost covers the inbound mirror: what the program tells the engine to
// dial for its callback server. The rule is netns symmetry — if the engine advertises itself
// to the program over loopback, the two share a namespace and the program can answer in kind.
func TestProgramAdvertiseHost(t *testing.T) {
	namer := fixedNamer{prefix: "pulumi-pod-p1-"}

	const podNet = "pulumi-pod-p1-net"

	t.Run("no override when the engine advertises loopback", func(t *testing.T) {
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "127.0.0.1")
		assert.Empty(t, programAdvertiseHost(namer, podNet),
			"shared netns: the SDK's loopback default is already correct")
	})

	t.Run("no override for other loopback spellings", func(t *testing.T) {
		for _, h := range []string{"localhost", "::1", "127.0.1.1"} {
			t.Setenv("PULUMI_POD_ADVERTISE_HOST", h)
			assert.Empty(t, programAdvertiseHost(namer, podNet), "%s is loopback", h)
		}
	})

	t.Run("advertises the program's own container name on a bridge", func(t *testing.T) {
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "pulumi-pod-p1-engine")
		assert.Equal(t, "pulumi-pod-p1-program", programAdvertiseHost(namer, podNet),
			"own netns: the engine must dial the program by a routable name")
	})

	// Engine-on-host (Option A): pod mode, but no pod network, so the default bridge has no
	// embedded DNS and the engine is outside it. A container name would be unresolvable, so
	// advertising one is worse than leaving the loopback default in place.
	t.Run("no override without a pod network", func(t *testing.T) {
		t.Setenv("PULUMI_POD_ADVERTISE_HOST", "host.docker.internal")
		assert.Empty(t, programAdvertiseHost(namer, ""),
			"no pod network means no name the engine could resolve")
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
