// Copyright 2022-2024, Pulumi Corporation.
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

package test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatches(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		n := n
		t.Run(strconv.Itoa(n), func(t *testing.T) {
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
	require.NoError(t, err)
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
