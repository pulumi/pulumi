// Copyright 2024, Pulumi Corporation.
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

package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

// findWorkspaceDir attempts to find the Bazel workspace directory by looking
// for the WORKSPACE or MODULE.bazel file in parent directories.
func findWorkspaceDir() string {
	// First check environment variables
	if wsDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); wsDir != "" {
		return wsDir
	}

	// For Bazel tests, TEST_SRCDIR points to runfiles root
	if testSrcDir := os.Getenv("TEST_SRCDIR"); testSrcDir != "" {
		// Check if we're in runfiles and can find the workspace
		workspaceDir := filepath.Join(testSrcDir, "_main")
		if fi, err := os.Stat(workspaceDir); err == nil && fi.IsDir() {
			return workspaceDir
		}
	}

	// Try to find workspace from current working directory
	cwd, err := os.Getwd()
	if err == nil {
		// Walk up looking for MODULE.bazel or WORKSPACE file
		dir := cwd
		for {
			if _, err := os.Stat(filepath.Join(dir, "MODULE.bazel")); err == nil {
				return dir
			}
			if _, err := os.Stat(filepath.Join(dir, "WORKSPACE")); err == nil {
				return dir
			}
			if _, err := os.Stat(filepath.Join(dir, "WORKSPACE.bazel")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// For regular Go tests, use runtime caller
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// This file is at pkg/codegen/testing/utils/testdata.go
		dir := filepath.Dir(filename)
		wsDir := filepath.Join(dir, "..", "..", "..", "..")
		if absDir, err := filepath.Abs(wsDir); err == nil {
			return absDir
		}
		return wsDir
	}

	// Last resort: current directory
	if cwd != "" {
		return cwd
	}
	return "."
}

// TestdataPath returns the path to the codegen testdata directory.
// It works for both Bazel tests and regular Go tests.
func TestdataPath() string {
	wsDir := findWorkspaceDir()
	return filepath.Join(wsDir, "tests", "testdata", "codegen")
}

// TestdataPathFromPkg returns the testdata path relative to a given package.
// This is useful for tests that expect the relative path format.
func TestdataPathFromPkg(pkgDir string) string {
	return TestdataPath()
}
