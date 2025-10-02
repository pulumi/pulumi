// Copyright 2021-2024, Pulumi Corporation.
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

package executable

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitGoPathShouldReturnExpected(t *testing.T) {
	t.Parallel()

	tt := []struct {
		path     string
		os       string
		expected int
	}{
		{
			path:     "/home/user/go:/usr/local/go",
			os:       "linux",
			expected: 2,
		},
		{
			path:     "C:/Users/User/Documents/go;C:/Workspace/go",
			os:       "windows",
			expected: 2,
		},
		{
			path:     "/home/user/go",
			os:       "linux",
			expected: 1,
		},
	}

	for _, test := range tt {
		paths := splitGoPath(test.path, test.os)
		if len(paths) != test.expected {
			t.Errorf("expected path length to be %d, got %d", test.expected, len(paths))
		}
	}
}

func TestFindExecutableShouldLookForExeAndCmdOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific test")
	}

	tempDir := t.TempDir()

	exePath := filepath.Join(tempDir, "mockprogram.exe")
	cmdPath := filepath.Join(tempDir, "mockprogram.cmd")
	ps1Path := filepath.Join(tempDir, "mockprogram.ps1")

	err := os.WriteFile(exePath, []byte("echo This is a mock .exe file"), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(cmdPath, []byte("echo This is a mock .cmd file"), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(ps1Path, []byte("echo This is a mock .ps1 file"), 0o755)
	require.NoError(t, err)

	t.Setenv("PATH", tempDir+";"+os.Getenv("PATH"))

	foundPath, err := FindExecutable("mockprogram")
	require.NoError(t, err)
	require.Equal(t, exePath, foundPath)

	err = os.Remove(exePath)
	require.NoError(t, err, "failed to remove mock .exe file")

	foundPath, err = FindExecutable("mockprogram")
	require.NoError(t, err)
	require.Equal(t, cmdPath, foundPath)

	err = os.Remove(cmdPath)

	foundPath, err = FindExecutable("mockprogram")
	require.NoError(t, err)
	require.Equal(t, ps1Path, foundPath)
}
