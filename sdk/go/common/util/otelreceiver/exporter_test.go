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

package otelreceiver

import (
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveFilePath(t *testing.T) {
	t.Parallel()

	t.Run("absolute path", func(t *testing.T) {
		t.Parallel()
		path, err := resolveFilePath("file:///tmp/traces.json")
		require.NoError(t, err)
		require.Equal(t, "/tmp/traces.json", path)
	})

	t.Run("tilde expands to home dir", func(t *testing.T) {
		t.Parallel()
		usr, err := user.Current()
		require.NoError(t, err)

		path, err := resolveFilePath("file://~/traces.json")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(usr.HomeDir, "traces.json"), path)
	})

	t.Run("relative path is made absolute", func(t *testing.T) {
		t.Parallel()
		path, err := resolveFilePath("file://relative/path.json")
		require.NoError(t, err)
		require.True(t, filepath.IsAbs(path))
	})

	t.Run("empty path returns error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveFilePath("file://")
		require.Error(t, err)
	})
}
