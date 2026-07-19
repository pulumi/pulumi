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
	"github.com/stretchr/testify/require"
)

func TestResolveRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		image   string
		version string
		tag     string
		want    string
		tagged  bool
		wantErr string
	}{
		{
			name: "version is the tag", image: "ghcr.io/acme/pack", version: "1.0.0",
			want: "ghcr.io/acme/pack:1.0.0", tagged: true,
		},
		{
			name: "tag override wins over version", image: "ghcr.io/acme/pack", version: "1.0.0", tag: "rc1",
			want: "ghcr.io/acme/pack:rc1", tagged: true,
		},
		{
			name: "explicit tag in image pins", image: "ghcr.io/acme/pack:dev", version: "1.0.0",
			want: "ghcr.io/acme/pack:dev", tagged: true,
		},
		{
			name: "explicit tag conflicts with override", image: "ghcr.io/acme/pack:dev", tag: "rc1",
			wantErr: "already has a tag",
		},
		{
			name: "registry port is not a tag", image: "registry.local:5000/pack", version: "1.0.0",
			want: "registry.local:5000/pack:1.0.0", tagged: true,
		},
		{
			name: "digest-pinned image passes through", image: "ghcr.io/acme/pack@sha256:abc",
			want: "ghcr.io/acme/pack@sha256:abc", tagged: true,
		},
		{
			name: "digest conflicts with override", image: "ghcr.io/acme/pack@sha256:abc", tag: "rc1",
			wantErr: "digest-pinned",
		},
		{
			name: "no tag anywhere falls back to latest", image: "ghcr.io/acme/pack",
			want: "ghcr.io/acme/pack:latest", tagged: false,
		},
		{
			name: "empty image errors", wantErr: "no image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ref, tagged, err := ResolveRef(tt.image, tt.version, tt.tag)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, ref)
			assert.Equal(t, tt.tagged, tagged)
		})
	}
}

func TestSplitRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ref, repo, tag, digest string
	}{
		{"ghcr.io/acme/pack", "ghcr.io/acme/pack", "", ""},
		{"ghcr.io/acme/pack:1.0.0", "ghcr.io/acme/pack", "1.0.0", ""},
		{"ghcr.io/acme/pack@sha256:abc", "ghcr.io/acme/pack", "", "sha256:abc"},
		{"ghcr.io/acme/pack:1.0.0@sha256:abc", "ghcr.io/acme/pack", "1.0.0", "sha256:abc"},
		{"localhost:5000/pack", "localhost:5000/pack", "", ""},
		{"localhost:5000/pack:1.0.0", "localhost:5000/pack", "1.0.0", ""},
		{"pack:1.0.0", "pack", "1.0.0", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			t.Parallel()
			repo, tag, digest := splitRef(tt.ref)
			assert.Equal(t, tt.repo, repo)
			assert.Equal(t, tt.tag, tag)
			assert.Equal(t, tt.digest, digest)
		})
	}
}
