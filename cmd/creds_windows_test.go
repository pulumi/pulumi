// +build
package cmd

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/registry"
)

// TestgetCredsFilePath ...
// Test to check creds FilePath ...
func TestgetCredsFilePath(t *testing.T) {

	want := registry.ExpandEnv("%APPDATA%")

	want = path.Join(want, pulumiSettingsFolder)

	got, err := getCredsFilePath()

	if err != nil {
		assert.Fail(t, "getCredsFilePath Failed")
		return
	}

	got, _ = filepath.Split(got)
	got, _ = filepath.Abs(got)

	assert.Equal(t, want, got)
}
