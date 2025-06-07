// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"io/fs"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies that the code in the pulumix package is up to date.
// If the parameters in pulumix/gen.go ever change,
// this test should be updated.
func TestPulumixIsUpToDate(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO[pulumi/pulumi#19675]: Fix this test on Windows
		t.Skip("Skipping tests on Windows")
	}
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
