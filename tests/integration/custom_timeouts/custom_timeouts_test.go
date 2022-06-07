//go:build python || all
// +build python all

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestCustomTimeouts(t *testing.T) {
	t.Parallel()

	opts := &integration.ProgramTestOptions{
		Dir: filepath.Join(".", "python", "success"),
		Dependencies: []string{
			filepath.Join("..", "..", "..", "sdk", "python", "env", "src"),
		},
		Quick:            true,
		DestroyOnCleanup: true,
	}
	integration.ProgramTest(t, opts)

	opts = &integration.ProgramTestOptions{
		Dir: filepath.Join(".", "python", "failure"),
		Dependencies: []string{
			filepath.Join("..", "..", "..", "sdk", "python", "env", "src"),
		},
		Quick:            true,
		DestroyOnCleanup: true,
		ExpectFailure:    true,
	}
	integration.ProgramTest(t, opts)
}
