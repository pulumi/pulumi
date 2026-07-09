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
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scriptRuntime(t *testing.T, script string) *Runtime {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o600))
	require.NoError(t, os.Chmod(path, 0o700))
	return &Runtime{Path: path, Name: "docker"}
}

func TestPullStreamsOutput(t *testing.T) {
	t.Parallel()
	rt := scriptRuntime(t, `#!/bin/sh
echo "Pulling from acme/pack"
`)
	var buf bytes.Buffer
	require.NoError(t, rt.Pull(t.Context(), "ghcr.io/acme/pack:1.0.0", &buf))
	assert.Contains(t, buf.String(), "Pulling from acme/pack")
}

func TestPullFailure(t *testing.T) {
	t.Parallel()
	rt := scriptRuntime(t, `#!/bin/sh
echo "pull access denied" >&2
exit 1
`)
	err := rt.Pull(t.Context(), "ghcr.io/acme/private:1.0.0", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghcr.io/acme/private:1.0.0")
}

func TestResolveDigest(t *testing.T) {
	t.Parallel()
	rt := scriptRuntime(t, `#!/bin/sh
echo '["ghcr.io/acme/pack@sha256:deadbeef"]'
`)
	ref, err := rt.ResolveDigest(t.Context(), "ghcr.io/acme/pack:1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/pack@sha256:deadbeef", ref)
}

func TestResolveDigestNotPushed(t *testing.T) {
	t.Parallel()
	rt := scriptRuntime(t, `#!/bin/sh
echo '[]'
`)
	_, err := rt.ResolveDigest(t.Context(), "ghcr.io/acme/pack:1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has it been pushed")
}

func TestResolveDigestPrefersMatchingRepository(t *testing.T) {
	t.Parallel()
	rt := scriptRuntime(t, `#!/bin/sh
echo '["docker.io/mirror/pack@sha256:aaaa","ghcr.io/acme/pack@sha256:bbbb"]'
`)
	ref, err := rt.ResolveDigest(t.Context(), "ghcr.io/acme/pack:1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/pack@sha256:bbbb", ref)
}

func TestHasPlatformManifestList(t *testing.T) {
	t.Parallel()
	rt := scriptRuntime(t, `#!/bin/sh
case "$1" in
  manifest) cat <<'EOF'
{"manifests":[{"platform":{"os":"linux","architecture":"amd64"}},{"platform":{"os":"linux","architecture":"arm64"}}]}
EOF
;;
esac
`)
	ok, err := rt.HasPlatform(t.Context(), "ref", "linux/amd64")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = rt.HasPlatform(t.Context(), "ref", "windows/amd64")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHasPlatformSingleImageFallback(t *testing.T) {
	t.Parallel()
	// manifest inspect returns a single-image manifest (no "manifests" list);
	// fall back to image inspect of the pulled image.
	rt := scriptRuntime(t, `#!/bin/sh
case "$1" in
  manifest) echo '{"schemaVersion":2,"config":{}}' ;;
  image) echo "linux/arm64" ;;
esac
`)
	ok, err := rt.HasPlatform(t.Context(), "ref", "linux/amd64")
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = rt.HasPlatform(t.Context(), "ref", "linux/arm64")
	require.NoError(t, err)
	assert.True(t, ok)
}
