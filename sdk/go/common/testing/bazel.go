// Copyright 2026, Pulumi Corporation.
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

package testing

import (
	"os"
	"path/filepath"
	"testing"
)

// IsInBazel returns true if running under Bazel test environment.
// Checks both BAZEL_TEST (set by .bazelrc) and TEST_SRCDIR (Bazel runfiles).
func IsInBazel() bool {
	return os.Getenv("BAZEL_TEST") != "" || os.Getenv("TEST_SRCDIR") != ""
}

// SkipInBazel skips the test if running under Bazel test environment.
func SkipInBazel(t testing.TB, reason string) {
	t.Helper()
	if IsInBazel() {
		t.Skipf("Skipping in Bazel environment: %s", reason)
	}
}

// RunfilesPath returns the path to a file in Bazel runfiles.
// pkgRelPath is relative to the workspace root (e.g., "sdk/go/auto/test/testproj").
// Falls back to the provided fallback path when not running under Bazel.
func RunfilesPath(pkgRelPath, fallback string) string {
	if runfilesDir := os.Getenv("RUNFILES_DIR"); runfilesDir != "" {
		candidate := filepath.Join(runfilesDir, "_main", pkgRelPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if testSrcDir := os.Getenv("TEST_SRCDIR"); testSrcDir != "" {
		candidate := filepath.Join(testSrcDir, "_main", pkgRelPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return fallback
}

// RunfilesPathResolved is like RunfilesPath but resolves symlinks.
// Useful for tools like go-git that cannot handle symlinked directories.
func RunfilesPathResolved(pkgRelPath, fallback string) string {
	if runfilesDir := os.Getenv("RUNFILES_DIR"); runfilesDir != "" {
		candidate := filepath.Join(runfilesDir, "_main", pkgRelPath)
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			return resolved
		}
	}
	if testSrcDir := os.Getenv("TEST_SRCDIR"); testSrcDir != "" {
		candidate := filepath.Join(testSrcDir, "_main", pkgRelPath)
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			return resolved
		}
	}
	return fallback
}
