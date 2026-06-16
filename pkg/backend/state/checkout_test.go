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

package state

import (
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkspaceWithSettings returns a workspace context whose New() yields a W backed by the given shared
// Settings pointer, so Set/Clear mutations are observable by a subsequent Get (mirroring the real
// per-process workspace cache).
func mockWorkspaceWithSettings(s *pkgWorkspace.Settings) *pkgWorkspace.MockContext {
	return &pkgWorkspace.MockContext{
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings { return s },
			}, nil
		},
	}
}

func TestCheckoutMarker(t *testing.T) {
	t.Parallel()

	t.Run("get returns nil when absent", func(t *testing.T) {
		t.Parallel()
		ws := mockWorkspaceWithSettings(&pkgWorkspace.Settings{})
		c, err := GetCheckout(ws, "org/proj/stack")
		require.NoError(t, err)
		assert.Nil(t, c)
	})

	t.Run("set then get round-trips and initializes a nil map", func(t *testing.T) {
		t.Parallel()
		ws := mockWorkspaceWithSettings(&pkgWorkspace.Settings{})
		marker := pkgWorkspace.Checkout{
			EnvRef:      "org/proj/env",
			Etag:        "etag-1",
			Revision:    3,
			FilePath:    "Pulumi.stack.local.yaml",
			ContentHash: "deadbeef",
			Imports:     []string{"org/proj/base"},
		}
		require.NoError(t, SetCheckout(ws, "org/proj/stack", marker))

		got, err := GetCheckout(ws, "org/proj/stack")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, marker, *got)
	})

	t.Run("clear removes the marker", func(t *testing.T) {
		t.Parallel()
		ws := mockWorkspaceWithSettings(&pkgWorkspace.Settings{
			Checkouts: map[string]pkgWorkspace.Checkout{"org/proj/stack": {EnvRef: "org/proj/env"}},
		})
		require.NoError(t, ClearCheckout(ws, "org/proj/stack"))

		got, err := GetCheckout(ws, "org/proj/stack")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("clear is a no-op when absent", func(t *testing.T) {
		t.Parallel()
		ws := mockWorkspaceWithSettings(&pkgWorkspace.Settings{})
		require.NoError(t, ClearCheckout(ws, "org/proj/stack"))
	})

	t.Run("markers are independent per stack", func(t *testing.T) {
		t.Parallel()
		ws := mockWorkspaceWithSettings(&pkgWorkspace.Settings{})
		require.NoError(t, SetCheckout(ws, "org/proj/a", pkgWorkspace.Checkout{EnvRef: "org/proj/env-a"}))
		require.NoError(t, SetCheckout(ws, "org/proj/b", pkgWorkspace.Checkout{EnvRef: "org/proj/env-b"}))

		require.NoError(t, ClearCheckout(ws, "org/proj/a"))

		a, err := GetCheckout(ws, "org/proj/a")
		require.NoError(t, err)
		assert.Nil(t, a)

		b, err := GetCheckout(ws, "org/proj/b")
		require.NoError(t, err)
		require.NotNil(t, b)
		assert.Equal(t, "org/proj/env-b", b.EnvRef)
	})
}
