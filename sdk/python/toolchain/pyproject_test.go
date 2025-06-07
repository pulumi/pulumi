// Copyright 2025, Pulumi Corporation.
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

package toolchain

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildablePackage(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO[pulumi/pulumi#19675]: Fix this test on Windows
		t.Skip("Skipping tests on Windows")
	}
	t.Parallel()
	tests := []struct {
		name               string
		content            string
		setupFunc          func(dir string) error
		isBuildablePackage bool
		errContains        string
	}{
		{
			name: "valid buildable package",
			content: `
				[project]
				name = "bananas"
				[build-system]
				requires = ["setuptools", "wheel"]
				build-backend = "setuptools.build_meta"`,
			isBuildablePackage: true,
		},
		{
			name: "non-buildable package",
			content: `
				[project]
				name = "bananas"`,
			isBuildablePackage: false,
		},
		{
			name: "no build-backend",
			content: `
				[project]
				name = "bananas"
				[build-system]
				requires = ["hatchling"]`,
			isBuildablePackage: false,
		},
		{
			name: "no name",
			content: `
				[project]
				[build-system]
				requires = ["setuptools"]
				build-backend = "setuptools.build_meta"`,
			isBuildablePackage: false,
		},
		{
			name:               "missing pyproject.toml",
			isBuildablePackage: false,
		},
		{
			name: "invalid TOML syntax",
			content: `
				[project
				name = "invalid"`,
			isBuildablePackage: false,
			errContains:        "unmarshaling pyproject.toml",
		},
		{
			name:    "permission denied",
			content: "something",
			setupFunc: func(dir string) error {
				// Make the file unreadable
				return os.Chmod(filepath.Join(dir, "pyproject.toml"), 0o000) // gosec
			},
			isBuildablePackage: false,
			errContains:        "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			pyprojectToml := filepath.Join(dir, "pyproject.toml")

			if tt.content != "" {
				err := os.WriteFile(pyprojectToml, []byte(tt.content), 0o600)
				require.NoError(t, err)
			}

			if tt.setupFunc != nil {
				err := tt.setupFunc(dir)
				require.NoError(t, err)
			}

			is, err := IsBuildablePackage(dir)

			if tt.errContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.isBuildablePackage, is)
			}
		})
	}
}
