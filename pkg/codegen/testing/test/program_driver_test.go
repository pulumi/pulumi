package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBatches(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		n := n
		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			t.Parallel()

			var combined []ProgramTest
			for i := 1; i <= n; i++ {
				combined = append(combined, ProgramTestBatch(i, n)...)
			}

			assert.ElementsMatch(t, PulumiPulumiProgramTests, combined)
		})
	}
}

// Checks that all synced tests from pulumi/yaml are in test list
func TestTranspiledExampleTestsCovered(t *testing.T) {
	t.Parallel()
	// Check that all synced tests from pulumi/yaml are in test list
	syncDir := filepath.Join("testdata", transpiledExamplesDir)
	untestedTranspiledExamples, err := getUntestedTranspiledExampleDirs(syncDir, PulumiPulumiYAMLProgramTests)
	assert.NoError(t, err)
	assert.Emptyf(t, untestedTranspiledExamples,
		"Untested examples in %s: %v", syncDir, untestedTranspiledExamples)
}

func getUntestedTranspiledExampleDirs(baseDir string, tests []ProgramTest) ([]string, error) {
	untested := make([]string, 0)
	testedDirs := make(map[string]bool)
	files, err := os.ReadDir(baseDir)
	if err != nil {
		return untested, err
	}

	for _, t := range tests {
		if strings.HasPrefix(t.Directory, transpiledExamplesDir) {
			dir := filepath.Base(t.Directory) + "-pp"
			testedDirs[dir] = true
		}
	}
	for _, f := range files {
		if _, ok := testedDirs[f.Name()]; !ok && f.IsDir() {
			untested = append(untested, f.Name())
		}
	}
	return untested, nil
}
