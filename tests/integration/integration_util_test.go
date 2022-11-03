// Copyright 2016-2022, Pulumi Corporation.
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

// The linter doesn't see the uses since the consumers are conditionally compiled tests.
//
// nolint:unused,deadcode,varcheck
package ints

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

const WindowsOS = "windows"

// assertPerfBenchmark implements the integration.TestStatsReporter interface, and reports test
// failures when a scenario exceeds the provided threshold.
type assertPerfBenchmark struct {
	T                  *testing.T
	MaxPreviewDuration time.Duration
	MaxUpdateDuration  time.Duration
}

func (t assertPerfBenchmark) ReportCommand(stats integration.TestCommandStats) {
	var maxDuration *time.Duration
	if strings.HasPrefix(stats.StepName, "pulumi-preview") {
		maxDuration = &t.MaxPreviewDuration
	}
	if strings.HasPrefix(stats.StepName, "pulumi-update") {
		maxDuration = &t.MaxUpdateDuration
	}

	if maxDuration != nil && *maxDuration != 0 {
		if stats.ElapsedSeconds < maxDuration.Seconds() {
			t.T.Logf(
				"Test step %q was under threshold. %.2fs (max %.2fs)",
				stats.StepName, stats.ElapsedSeconds, maxDuration.Seconds())
		} else {
			t.T.Errorf(
				"Test step %q took longer than expected. %.2fs vs. max %.2fs",
				stats.StepName, stats.ElapsedSeconds, maxDuration.Seconds())
		}
	}
}

func testComponentSlowLocalProvider(t *testing.T) integration.LocalDependency {
	return integration.LocalDependency{
		Package: "testcomponent",
		Path:    filepath.Join("construct_component_slow", "testcomponent"),
	}
}

func testComponentProviderSchema(t *testing.T, path string) {
	t.Parallel()

	runComponentSetup(t, "component_provider_schema")

	tests := []struct {
		name          string
		env           []string
		version       int32
		expected      string
		expectedError string
	}{
		{
			name:     "Default",
			expected: "{}",
		},
		{
			name:     "Schema",
			env:      []string{"INCLUDE_SCHEMA=true"},
			expected: `{"hello": "world"}`,
		},
		{
			name:          "Invalid Version",
			version:       15,
			expectedError: "unsupported schema version 15",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Start the plugin binary.
			cmd := exec.Command(path, "ignored")
			cmd.Env = append(os.Environ(), test.env...)
			stdout, err := cmd.StdoutPipe()
			assert.NoError(t, err)
			err = cmd.Start()
			assert.NoError(t, err)
			defer func() {
				// Ignore the error as it may fail with access denied on Windows.
				cmd.Process.Kill() // nolint: errcheck
			}()

			// Read the port from standard output.
			reader := bufio.NewReader(stdout)
			bytes, err := reader.ReadBytes('\n')
			assert.NoError(t, err)
			port := strings.TrimSpace(string(bytes))

			// Create a connection to the server.
			conn, err := grpc.Dial("127.0.0.1:"+port, grpc.WithInsecure(), rpcutil.GrpcChannelOptions())
			assert.NoError(t, err)
			client := pulumirpc.NewResourceProviderClient(conn)

			// Call GetSchema and verify the results.
			resp, err := client.GetSchema(context.Background(), &pulumirpc.GetSchemaRequest{Version: test.version})
			if test.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				assert.Equal(t, test.expected, resp.GetSchema())
			}
		})
	}
}

// Test remote component inputs properly handle unknowns.
func testConstructUnknown(t *testing.T, lang string, dependencies ...string) {
	t.Parallel()

	const testDir = "construct_component_unknown"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			localProviders :=
				[]integration.LocalDependency{
					{Package: "testprovider", Path: buildTestProvider(t, filepath.Join("..", "testprovider"))},
					{Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir)},
				}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:                    filepath.Join(testDir, lang),
				Dependencies:           dependencies,
				LocalProviders:         localProviders,
				SkipRefresh:            true,
				SkipPreview:            false,
				SkipUpdate:             true,
				SkipExportImport:       true,
				SkipEmptyPreviewUpdate: true,
				Quick:                  false,
			})
		})
	}
}

// Test methods properly handle unknowns.
func testConstructMethodsUnknown(t *testing.T, lang string, dependencies ...string) {
	t.Parallel()

	const testDir = "construct_component_methods_unknown"
	runComponentSetup(t, testDir)
	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		test := test

		t.Run(test.componentDir, func(t *testing.T) {
			localProviders :=
				[]integration.LocalDependency{
					{Package: "testprovider", Path: buildTestProvider(t, filepath.Join("..", "testprovider"))},
					{Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir)},
				}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:                    filepath.Join(testDir, lang),
				Dependencies:           dependencies,
				LocalProviders:         localProviders,
				SkipRefresh:            true,
				SkipPreview:            false,
				SkipUpdate:             true,
				SkipExportImport:       true,
				SkipEmptyPreviewUpdate: true,
				Quick:                  false,
			})
		})
	}
}

func buildTestProvider(t *testing.T, providerDir string) string {
	fn := func() {
		providerName := "pulumi-resource-testprovider"
		if runtime.GOOS == "windows" {
			providerName += ".exe"
		}

		_, err := os.Stat(filepath.Join(providerDir, providerName))
		if err == nil {
			return
		} else if errors.Is(err, os.ErrNotExist) {
			// Not built yet, continue.
		} else {
			t.Fatalf("Unexpected error building test provider: %v", err)
		}

		cmd := exec.Command("go", "build", "-o", providerName)
		cmd.Dir = providerDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			contract.AssertNoErrorf(err, "failed to run setup script: %v", string(output))
		}
	}
	lockfile := filepath.Join(providerDir, ".lock")
	timeout := 10 * time.Minute
	synchronouslyDo(t, lockfile, timeout, fn)

	// Allows us to drop this in in places where providerDir was used:
	return providerDir
}

func runComponentSetup(t *testing.T, testDir string) {
	ptesting.YarnInstallMutex.Lock()
	defer ptesting.YarnInstallMutex.Unlock()

	setupFilename, err := filepath.Abs("component_setup.sh")
	contract.AssertNoError(err)
	// even for Windows, we want forward slashes as bash treats backslashes as escape sequences.
	setupFilename = filepath.ToSlash(setupFilename)
	fn := func() {
		cmd := exec.Command("bash", setupFilename)
		cmd.Dir = testDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			contract.AssertNoErrorf(err, "failed to run setup script: %v", string(output))
		}
	}
	lockfile := filepath.Join(testDir, ".lock")
	timeout := 10 * time.Minute
	synchronouslyDo(t, lockfile, timeout, fn)
}

func synchronouslyDo(t *testing.T, lockfile string, timeout time.Duration, fn func()) {
	mutex := fsutil.NewFileMutex(lockfile)
	defer func() {
		assert.NoError(t, mutex.Unlock())
	}()

	lockWait := make(chan struct{}, 1)
	go func() {
		for {
			if err := mutex.Lock(); err != nil {
				time.Sleep(1 * time.Second)
				continue
			} else {
				break
			}
		}

		fn()
		lockWait <- struct{}{}
	}()

	select {
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for lock on %s", lockfile)
	case <-lockWait:
		// waited for fn, success.
	}
}

// Test methods that create resources.
func testConstructMethodsResources(t *testing.T, lang string, dependencies ...string) {
	t.Parallel()

	const testDir = "construct_component_methods_resources"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			localProviders :=
				[]integration.LocalDependency{
					{Package: "testprovider", Path: buildTestProvider(t, filepath.Join("..", "testprovider"))},
					{Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir)},
				}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:            filepath.Join(testDir, lang),
				Dependencies:   dependencies,
				LocalProviders: localProviders,
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.NotNil(t, stackInfo.Deployment)
					assert.Equal(t, 6, len(stackInfo.Deployment.Resources))
					var hasExpectedResource bool
					var result string
					for _, res := range stackInfo.Deployment.Resources {
						if res.URN.Name().String() == "myrandom" {
							hasExpectedResource = true
							result = res.Outputs["result"].(string)
							assert.Equal(t, float64(10), res.Inputs["length"])
							assert.Equal(t, 10, len(result))
						}
					}
					assert.True(t, hasExpectedResource)
					assert.Equal(t, result, stackInfo.Outputs["result"])
				},
			})
		})
	}
}

// Test failures returned from methods are observed.
func testConstructMethodsErrors(t *testing.T, lang string, dependencies ...string) {
	t.Parallel()

	const testDir = "construct_component_methods_errors"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			expectedError := "the failure reason (the failure property)"

			localProvider := integration.LocalDependency{
				Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir),
			}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:            filepath.Join(testDir, lang),
				Dependencies:   dependencies,
				LocalProviders: []integration.LocalDependency{localProvider},
				Quick:          true,
				Stderr:         stderr,
				ExpectFailure:  true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					output := stderr.String()
					assert.Contains(t, output, expectedError)
				},
			})
		})
	}
}

func testConstructOutputValues(t *testing.T, lang string, dependencies ...string) {
	t.Parallel()

	const testDir = "construct_component_output_values"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			localProviders :=
				[]integration.LocalDependency{
					{Package: "testprovider", Path: buildTestProvider(t, filepath.Join("..", "testprovider"))},
					{Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir)},
				}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:            filepath.Join(testDir, lang),
				Dependencies:   dependencies,
				LocalProviders: localProviders,
				Quick:          true,
			})
		})
	}
}

var previewSummaryRegex = regexp.MustCompile(
	`{\s+"steps": \[[\s\S]+],\s+"duration": \d+,\s+"changeSummary": {[\s\S]+}\s+}`)

func assertOutputContainsEvent(t *testing.T, evt apitype.EngineEvent, output string) {
	evtJSON := bytes.Buffer{}
	encoder := json.NewEncoder(&evtJSON)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(evt)
	assert.NoError(t, err)
	assert.Contains(t, output, evtJSON.String())
}

// printfTestValidation is used by the TestPrintfXYZ test cases in the language-specific test
// files. It validates that there are a precise count of expected stdout/stderr lines in the test output.
func printfTestValidation(t *testing.T, stack integration.RuntimeValidationStackInfo) {
	var foundStdout int
	var foundStderr int
	for _, ev := range stack.Events {
		if de := ev.DiagnosticEvent; de != nil {
			if strings.HasPrefix(de.Message, fmt.Sprintf("Line %d", foundStdout)) {
				foundStdout++
			} else if strings.HasPrefix(de.Message, fmt.Sprintf("Errln %d", foundStderr+10)) {
				foundStderr++
			}
		}
	}
	assert.Equal(t, 11, foundStdout)
	assert.Equal(t, 11, foundStderr)
}
