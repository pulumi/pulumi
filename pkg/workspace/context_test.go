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

package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestNewContextFromReadsProjectAtDir verifies a dir-scoped Context detects the
// project by searching upwards from its directory, independent of the process
// working directory (the test never chdirs into the project).
func TestNewContextFromReadsProjectAtDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "Pulumi.yaml"),
		[]byte("name: ctx-test\nruntime: yaml\n"), 0o600,
	))
	nested := filepath.Join(root, "sub", "dir")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	proj, dir, err := Instance.ReadProject(nested)
	require.NoError(t, err)
	assert.Equal(t, "ctx-test", string(proj.Name))
	// The project root is where Pulumi.yaml lives, possibly symlink-resolved on
	// platforms where the temp dir is behind a symlink.
	realRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	assert.Equal(t, realRoot, realDir)
}

func TestNewContextFromNoProject(t *testing.T) {
	t.Parallel()

	// An empty directory (outside any Pulumi project) yields ErrProjectNotFound,
	// matching Instance's sentinel so callers' errors.Is checks behave the same.
	_, _, err := Instance.ReadProject(t.TempDir())
	require.Error(t, err)
	assert.True(t, errors.Is(err, workspace.ErrProjectNotFound))
}
