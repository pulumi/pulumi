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

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// regenerate runs the in-process generator with the shared fixture and the
// testing boilerplate, returning the output directory it wrote into.
//
// Tests call this in preference to shelling out to `go run` because it is
// faster and lets us assert on the exit code programmatically.
func regenerate(t *testing.T) string {
	t.Helper()

	dir, err := generatorDir()
	if err != nil {
		t.Fatalf("resolving generator directory: %v", err)
	}

	fixture := filepath.Join(dir, "tests", "fixture.json")
	os.Args = []string{"automation-codegen", fixture, "boilerplate/testing"}
	if code := run(); code != 0 {
		t.Fatalf("generator returned non-zero exit code: %d", code)
	}

	outDir, err := defaultOutputDir()
	if err != nil {
		t.Fatalf("resolving output directory: %v", err)
	}
	return outDir
}

// TestDeterminism verifies that two back-to-back runs of the generator
// produce byte-identical output. Non-determinism usually means map iteration
// leaked through somewhere it shouldn't have.
//
// The tests in this file share the output directory and global os.Args;
// they must run serially.
//
//nolint:paralleltest
func TestDeterminism(t *testing.T) {
	outDir := regenerate(t)
	first, err := readTree(outDir)
	if err != nil {
		t.Fatalf("reading first generation: %v", err)
	}

	// Remove and regenerate to exercise the full pipeline again.
	if err := os.RemoveAll(outDir); err != nil {
		t.Fatalf("cleaning up first generation: %v", err)
	}
	regenerate(t)
	second, err := readTree(outDir)
	if err != nil {
		t.Fatalf("reading second generation: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("different file sets: %d vs %d", len(first), len(second))
	}
	for path, content := range first {
		other, ok := second[path]
		if !ok {
			t.Errorf("second run missing %s", path)
			continue
		}
		if !bytes.Equal(content, other) {
			t.Errorf("content differs for %s", path)
		}
	}
}

// TestGeneratedCompiles makes sure the generated output is a valid Go
// program. We lean on `go build`: the test passes iff the compiler is
// happy with everything under output/.
//
// Shares the output directory with the other tests, so no t.Parallel.
//
//nolint:paralleltest
func TestGeneratedCompiles(t *testing.T) {
	outDir := regenerate(t)

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = outDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}

// TestRuntimeCommands regenerates the output and then invokes the tagged
// runtime test package as a subprocess. The subprocess imports the freshly
// generated output packages and asserts that each command produces the
// expected CLI invocation.
//
// Subprocess is needed because the runtime tests cannot be compiled into
// this test binary — the output packages they import may not exist until
// the generator runs.
//
// Shares the output directory with the other tests, so no t.Parallel.
//
//nolint:paralleltest
func TestRuntimeCommands(t *testing.T) {
	regenerate(t)

	dir, err := generatorDir()
	if err != nil {
		t.Fatalf("resolving generator directory: %v", err)
	}

	cmd := exec.Command("go", "test", "-tags=automation_runtime", "-v", "./...")
	cmd.Dir = filepath.Join(dir, "tests", "runtime")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime tests failed: %v\n%s", err, out)
	}
	t.Log(string(out))
}

// readTree walks root and returns a map of relative path -> file contents.
func readTree(root string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
