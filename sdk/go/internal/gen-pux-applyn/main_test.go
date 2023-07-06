package main

import (
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies that the code in the pulumix package is up to date.
// If the parameters in pulumix/gen.go ever change,
// this test should be updated.
func TestPulumixIsUpToDate(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	require.NoError(t, run(&params{Dir: outDir}))

	// Compare the generated code to the code in pulumix.
	expected := os.DirFS(outDir)
	actual := os.DirFS("../../pulumix")

	err := fs.WalkDir(expected, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || err != nil {
			return err
		}

		// Path is relative to the root of the fs.FS
		// so we can re-use it for both FSes.

		wantFile, err := fs.ReadFile(expected, path)
		require.NoError(t, err)

		haveFile, err := fs.ReadFile(actual, path)
		require.NoError(t, err)

		assert.Equal(t, string(wantFile), string(haveFile),
			"path: %v", path)

		return nil
	})
	require.NoError(t, err)

	// Provide information on how to fix this failure.
	if t.Failed() {
		t.Errorf("Generated code in pulumix is out of date.")
		t.Errorf("To fix this, run `go generate` in the pulumix directory.")
	}
}
