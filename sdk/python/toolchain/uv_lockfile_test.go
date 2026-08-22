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

package toolchain

import (
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

// A synthetic workspace lock with two members that share transitive dependencies. member-a additionally
// pulls in a package via a local path source and has a dev-group dependency.
const workspaceLock = `
version = 1

[[package]]
name = "ws-root"
version = "0.1.0"
source = { virtual = "." }

[[package]]
name = "member-a"
version = "0.1.0"
source = { virtual = "members/a" }
dependencies = [
    { name = "requests" },
    { name = "pulumi-onepassword" },
]

[package.dev-dependencies]
dev = [
    { name = "iniconfig" },
]

[[package]]
name = "member-b"
version = "0.1.0"
source = { virtual = "members/b" }
dependencies = [
    { name = "packaging" },
]

[[package]]
name = "requests"
version = "2.32.3"
source = { registry = "https://pypi.org/simple" }
dependencies = [
    { name = "idna" },
]

[[package]]
name = "idna"
version = "3.7"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "pulumi-onepassword"
version = "1.1.3"
source = { path = "wheels/pulumi_onepassword-1.1.3-py3-none-any.whl" }

[[package]]
name = "packaging"
version = "24.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "iniconfig"
version = "2.0.0"
source = { registry = "https://pypi.org/simple" }
`

func decodeWorkspaceLock(t *testing.T) uvLockFile {
	t.Helper()
	var lock uvLockFile
	_, err := toml.Decode(workspaceLock, &lock)
	require.NoError(t, err)
	return lock
}

func TestUvLockScopeToMember(t *testing.T) {
	t.Parallel()

	lockDir := filepath.FromSlash("/ws")

	t.Run("member-b only sees its own transitive deps", func(t *testing.T) {
		t.Parallel()
		lock := decodeWorkspaceLock(t)
		packages, ok := lock.scopeToMember(lockDir, filepath.Join(lockDir, "members", "b"), true /* transitive */)
		require.True(t, ok)

		got := map[string]string{}
		for _, p := range packages {
			got[p.Name] = p.Version
		}
		require.Equal(t, map[string]string{"packaging": "24.0"}, got,
			"member B must not see member A's dependencies (requests, pulumi-onepassword, idna, iniconfig)")
	})

	t.Run("member-a sees runtime, path, transitive and dev deps", func(t *testing.T) {
		t.Parallel()
		lock := decodeWorkspaceLock(t)
		packages, ok := lock.scopeToMember(lockDir, filepath.Join(lockDir, "members", "a"), true /* transitive */)
		require.True(t, ok)

		got := map[string]string{}
		for _, p := range packages {
			got[p.Name] = p.Version
		}
		require.Equal(t, map[string]string{
			"requests":           "2.32.3", // direct runtime dep
			"idna":               "3.7",    // transitive dep of requests
			"pulumi-onepassword": "1.1.3",  // direct dep from a local path source (retains its version)
			"iniconfig":          "2.0.0",  // dev-group dep, which uv sync installs by default
		}, got)
	})

	t.Run("project root maps to '.'", func(t *testing.T) {
		t.Parallel()
		lock := decodeWorkspaceLock(t)
		// ws-root has no dependencies of its own.
		packages, ok := lock.scopeToMember(lockDir, lockDir, true /* transitive */)
		require.True(t, ok)
		require.Empty(t, packages)
	})

	t.Run("unknown directory falls back", func(t *testing.T) {
		t.Parallel()
		lock := decodeWorkspaceLock(t)
		_, ok := lock.scopeToMember(lockDir, filepath.Join(lockDir, "members", "does-not-exist"), true)
		require.False(t, ok, "an unlocatable member signals the caller to fall back to the whole lock")
	})
}

func TestUvLockVirtualPackages(t *testing.T) {
	t.Parallel()
	lock := decodeWorkspaceLock(t)
	require.Equal(t, map[string]bool{
		"ws-root":  true,
		"member-a": true,
		"member-b": true,
	}, lock.virtualPackages())
}
