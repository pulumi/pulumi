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

// CacheVolumeName must be stable (so a cache reuses the same volume across builds and
// pods) and a valid docker volume name (no slashes), with the recognizable prefix.
func TestCacheVolumeName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"/buildcache":           "pulumi-oci-buildcache-buildcache",
		"/root/.cache/go-build": "pulumi-oci-buildcache-root-.cache-go-build",
		"/go/pkg/mod":           "pulumi-oci-buildcache-go-pkg-mod",
		"nix/store":             "pulumi-oci-buildcache-nix-store",
	}
	for path, want := range cases {
		assert.Equal(t, want, CacheVolumeName(path), "path %q", path)
		// Stability: same input, same output.
		assert.Equal(t, CacheVolumeName(path), CacheVolumeName(path))
	}
}
