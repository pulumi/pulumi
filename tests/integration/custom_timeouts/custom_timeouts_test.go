package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func TestCustomTimeouts(t *testing.T) {
	opts := &integration.ProgramTestOptions{
		Dir: filepath.Join(".", "python"),
		Dependencies: []string{
			filepath.Join("..", "..", "..", "sdk", "python", "env", "src"),
		},
		Quick:         true,
		DebugLogLevel: 9,
	}
	integration.ProgramTest(t, opts)
}
