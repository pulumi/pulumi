// +build
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/registry"
)

// TestgetCredsFilePath ...
// Test to check creds FilePath ...
func TestgetCredsFilePath(t *testing.T) {
	want := registry.ExpandEnv("%APPDATA%")

	got, err := getCredsFilePath()

	if err != nil {
		assert.Equal(t, want, got)
	}

	assert.Equal(t, want, got)
}
