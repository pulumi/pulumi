// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package npm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindWorkspaceRoot(t *testing.T) {
	t.Parallel()

	root, err := FindWorkspaceRoot(filepath.Join("testdata", "workspace", "project"))

	require.NoError(t, err)
	require.Equal(t, filepath.Join("testdata", "workspace"), root)
}

func TestFindWorkspaceRootNotInWorkspace(t *testing.T) {
	t.Parallel()

	_, err := FindWorkspaceRoot(filepath.Join("testdata", "nested", "project"))

	require.ErrorIs(t, err, ErrNotInWorkspace)
}

func TestFindWorkspaceRootYarnExtended(t *testing.T) {
	t.Parallel()

	root, err := FindWorkspaceRoot(filepath.Join("testdata", "workspace-extended", "project"))

	require.NoError(t, err)
	require.Equal(t, filepath.Join("testdata", "workspace-extended"), root)
}

func TestFindWorkspaceRootNested(t *testing.T) {
	t.Parallel()

	root, err := FindWorkspaceRoot(filepath.Join("testdata", "workspace-nested", "project", "dist"))

	require.NoError(t, err)
	require.Equal(t, filepath.Join("testdata", "workspace-nested"), root)
}

func TestFindWorkspaceRootFileArgument(t *testing.T) {
	t.Parallel()

	// Use a file as the argument to FindWorkspaceRoot instead of a directory.
	root, err := FindWorkspaceRoot(filepath.Join("testdata", "workspace-nested", "project", "dist", "index.js"))

	require.NoError(t, err)
	require.Equal(t, filepath.Join("testdata", "workspace-nested"), root)
}

func TestNotAWorkspace(t *testing.T) {
	t.Parallel()

	_, err := FindWorkspaceRoot(filepath.Join("testdata", "not-a-workspace"))

	require.ErrorIs(t, err, ErrNotInWorkspace)
}

func TestFindWorkspaceRootPNPM(t *testing.T) {
	t.Parallel()

	root, err := FindWorkspaceRoot(filepath.Join("testdata", "pnpm-workspace", "project"))

	require.NoError(t, err)
	require.Equal(t, filepath.Join("testdata", "pnpm-workspace"), root)
}
