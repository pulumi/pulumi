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

package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var testProvider struct {
	once sync.Once
	dir  string
	bin  string
	err  error
}

// buildTestProvider builds tests/testprovider once per test process.
//
// When a test points the engine at the testprovider source directory instead,
// the engine starts the plugin through the Go language host, which runs
// `go build` every time the engine boots the provider: once per operation per
// test. On CI runners those concurrent toolchain invocations dominate the
// test's wall time, so tests should prefer the prebuilt binary.
func buildTestProvider(t testing.TB) {
	testProvider.once.Do(func() {
		// Not t.TempDir(): that is removed when the test that happens to build
		// the provider finishes, while every later test still needs the binary.
		dir, err := os.MkdirTemp("", "pulumi-testprovider") //nolint:usetesting
		if err != nil {
			testProvider.err = err
			return
		}

		binary := filepath.Join(dir, "pulumi-resource-testprovider")
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		// The test's working directory is inside the tests module, so the
		// import path resolves without knowing where this file is on disk.
		cmd := exec.Command("go", "build", "-o", binary, "github.com/pulumi/pulumi/tests/testprovider")
		if output, err := cmd.CombinedOutput(); err != nil {
			testProvider.err = fmt.Errorf("building testprovider: %w\n%s", err, output)
			return
		}
		testProvider.dir = dir
		testProvider.bin = binary
	})
	require.NoError(t, testProvider.err)
}

// TestProviderDir returns the directory containing a prebuilt
// pulumi-resource-testprovider binary, for use as an
// integration.LocalDependency path: the engine resolves a project plugin path
// by looking the binary up inside the directory.
func TestProviderDir(t testing.TB) string {
	buildTestProvider(t)
	return testProvider.dir
}

// TestProvider returns the path of a prebuilt pulumi-resource-testprovider
// binary, for commands like `pulumi package add` that take the plugin binary
// itself and infer the package name from its filename.
func TestProvider(t testing.TB) string {
	buildTestProvider(t)
	return testProvider.bin
}
