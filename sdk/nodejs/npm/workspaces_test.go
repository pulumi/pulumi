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
