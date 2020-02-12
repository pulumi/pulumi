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
		Quick: true,
		EditDirs: []integration.EditDir{
			{
				Dir:      "step1",
				Additive: true,
			},
			{
				Dir:      "step2",
				Additive: true,
			},
			{
				Dir:      "step3",
				Additive: true,
			},
		},
	}
	integration.ProgramTest(t, opts)
}
