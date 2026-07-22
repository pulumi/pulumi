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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pullRecorderPod is a fakePod that serves a canned required-packages manifest from
// ReadImageFile and records the refs PullImage is asked to fetch. ImageExists is
// inherited from fakePod (its imageExists field decides present-vs-absent).
type pullRecorderPod struct {
	fakePod
	manifest []byte
	mu       sync.Mutex
	pulled   []string
}

func (p *pullRecorderPod) ReadImageFile(context.Context, string, string) ([]byte, error) {
	return p.manifest, nil
}

func (p *pullRecorderPod) PullImage(_ context.Context, ref string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pulled = append(p.pulled, ref)
	return nil
}

// prefetchNow reads the program image's manifest and warms each declared provider image;
// a versioned entry resolves to a ref and is pulled, a versionless one cannot and is
// skipped (lazy discovery still starts it when needed).
func TestPrefetchNowPullsVersionedImagesSkipsVersionless(t *testing.T) {
	t.Parallel()
	pod := &pullRecorderPod{manifest: []byte(`{"plugins":[
		{"resource":true,"name":"random","version":"4.16.0"},
		{"resource":true,"name":"noversion"},
		{"resource":true,"name":"tls","version":"5.0.0"}]}`)}
	h := &containerHost{pod: pod, programImage: "prog:latest"}

	h.prefetchNow(t.Context())

	assert.ElementsMatch(t, []string{
		DefaultPublicRegistry + "/pulumi/pulumi-provider-random:v4.16.0",
		DefaultPublicRegistry + "/pulumi/pulumi-provider-tls:v5.0.0",
	}, pod.pulled, "versioned entries are warmed; a versionless entry has no resolvable ref and is skipped")
}

// prefetch is gated on a program image to read: with none there is nothing to warm, so it
// must spawn no work (it returns before touching the pod — fakePod.ReadImageFile would panic).
func TestPrefetchGuardIsNoopWithoutProgramImage(t *testing.T) {
	t.Parallel()
	require.NotPanics(t, func() {
		(&containerHost{pod: fakePod{}, programImage: ""}).prefetch()
	})
}

// pullImage (the seam shared by prefetch and lazy ensureImage) pulls an absent image and
// skips a present one.
func TestPullImagePullsWhenAbsentSkipsWhenPresent(t *testing.T) {
	t.Parallel()

	absent := &pullRecorderPod{}
	require.NoError(t, (&containerHost{pod: absent}).pullImage(t.Context(), "reg/pulumi-provider-x:v1"))
	assert.Equal(t, []string{"reg/pulumi-provider-x:v1"}, absent.pulled, "an absent image is pulled")

	present := &pullRecorderPod{fakePod: fakePod{imageExists: true}}
	require.NoError(t, (&containerHost{pod: present}).pullImage(t.Context(), "reg/pulumi-provider-x:v1"))
	assert.Empty(t, present.pulled, "a present image is not pulled")
}
