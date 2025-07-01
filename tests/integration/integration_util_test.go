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

package ints

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const WindowsOS = "windows"

func testComponentSlowLocalProvider(t *testing.T) integration.LocalDependency {
	return integration.LocalDependency{
		Package: "testcomponent",
		Path:    filepath.Join("construct_component_slow", "testcomponent"),
	}
}

func testComponentProviderSchema(t *testing.T, path string) {
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
			require.NoError(t, err)
			err = cmd.Start()
			require.NoError(t, err)
			defer func() {
				// Ignore the error as it may fail with access denied on Windows.
				cmd.Process.Kill() //nolint:errcheck
			}()

			// Read the port from standard output.
			reader := bufio.NewReader(stdout)
			bytes, err := reader.ReadBytes('\n')
			require.NoError(t, err)
			port := strings.TrimSpace(string(bytes))

			// Create a connection to the server.
			conn, err := grpc.NewClient(
				"127.0.0.1:"+port,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				rpcutil.GrpcChannelOptions(),
			)
			require.NoError(t, err)
			client := pulumirpc.NewResourceProviderClient(conn)

			// Call GetSchema and verify the results.
			resp, err := client.GetSchema(context.Background(), &pulumirpc.GetSchemaRequest{Version: test.version})
			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.Equal(t, test.expected, resp.GetSchema())
			}
		})
	}
}

// Test remote component inputs properly handle unknowns.
func testConstructUnknown(t *testing.T, lang string, dependencies ...string) {
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
			localProviders := []integration.LocalDependency{
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
			localProviders := []integration.LocalDependency{
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

func runComponentSetup(t *testing.T, testDir string) {
	ptesting.YarnInstallMutex.Lock()
	defer ptesting.YarnInstallMutex.Unlock()

	setupFilename, err := filepath.Abs("component_setup.sh")
	require.NoError(t, err, "could not determine absolute path")
	// Even for Windows, we want forward slashes as bash treats backslashes as escape sequences.
	setupFilename = filepath.ToSlash(setupFilename)

	synchronouslyDo(t, filepath.Join(testDir, ".lock"), 10*time.Minute, func() {
		out := iotest.LogWriter(t)

		cmd := exec.Command("bash", "-x", setupFilename)
		cmd.Dir = testDir
		cmd.Stdout = out
		cmd.Stderr = out
		err := cmd.Run()

		// This runs in a separate goroutine, so don't use 'require'.
		require.NoError(t, err, "failed to run setup script")
	})

	// The function above runs in a separate goroutine
	// so it can't halt test execution.
	// Verify that it didn't fail separately
	// and halt execution if it did.
	require.False(t, t.Failed(), "component setup failed")
}

func synchronouslyDo(t testing.TB, lockfile string, timeout time.Duration, fn func()) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	lockWait := make(chan struct{})
	go func() {
		mutex := fsutil.NewFileMutex(lockfile)

		// ctx.Err will be non-nil when the context finishes
		// either because it timed out or because it got canceled.
		for ctx.Err() == nil {
			if err := mutex.Lock(); err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			defer func() {
				require.NoError(t, mutex.Unlock())
			}()
			break
		}

		// Context may hav expired
		// by the time we acquired the lock.
		if ctx.Err() == nil {
			fn()
			close(lockWait)
		}
	}()

	select {
	case <-ctx.Done():
		t.Fatalf("timed out waiting for lock on %s", lockfile)
	case <-lockWait:
		// waited for fn, success.
	}
}

// Verifies that if a file lock is already acquired,
// synchronouslyDo is able to time out properly.
func TestSynchronouslyDo_timeout(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "foo")
	mu := fsutil.NewFileMutex(path)
	require.NoError(t, mu.Lock())
	defer func() {
		require.NoError(t, mu.Unlock())
	}()

	fakeT := nonfatalT{T: t}
	synchronouslyDo(&fakeT, path, 10*time.Millisecond, func() {
		t.Errorf("timed-out operation should not be called")
	})

	assert.True(t, fakeT.fatal, "must have a fatal failure")
	if assert.Len(t, fakeT.messages, 1) {
		assert.Contains(t, fakeT.messages[0], "timed out waiting")
	}
}

// nonfatalT wraps a testing.T to capture fatal errors.
type nonfatalT struct {
	*testing.T

	mu       sync.Mutex
	fatal    bool
	messages []string
}

func (t *nonfatalT) Fatalf(msg string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.fatal = true
	t.messages = append(t.messages, fmt.Sprintf(msg, args...))
}

// Test methods that create resources.
func testConstructMethodsResources(t *testing.T, lang string, dependencies ...string) {
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
			localProviders := []integration.LocalDependency{
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
						if res.URN.Name() == "myrandom" {
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

// Tests methods work when there is an explicit provider for another provider set on the component.
func testConstructMethodsProvider(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_methods_provider"
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
			localProvider := integration.LocalDependency{
				Package: "testcomponent", Path: filepath.Join(testDir, test.componentDir),
			}
			testProvider := integration.LocalDependency{
				Package: "testprovider", Path: filepath.Join("..", "testprovider"),
			}
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:            filepath.Join(testDir, lang),
				Dependencies:   dependencies,
				LocalProviders: []integration.LocalDependency{localProvider, testProvider},
				Quick:          true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					assert.Equal(t, "Hello World, Alice!", stackInfo.Outputs["message1"])
					assert.Equal(t, "Hi There, Bob!", stackInfo.Outputs["message2"])
				},
			})
		})
	}
}

func testConstructOutputValues(t *testing.T, lang string, dependencies ...string) {
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
			localProviders := []integration.LocalDependency{
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
	require.NoError(t, err)
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

func testConstructProviderExplicit(t *testing.T, lang string, dependencies []string) {
	const testDir = "construct_component_provider_explicit"
	runComponentSetup(t, testDir)

	localProvider := integration.LocalDependency{
		Package: "testcomponent", Path: filepath.Join(testDir, "testcomponent-go"),
	}
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:            filepath.Join(testDir, lang),
		Dependencies:   dependencies,
		LocalProviders: []integration.LocalDependency{localProvider},
		Quick:          true,
		NoParallel:     true, // already called by tests
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Equal(t, "hello world", stackInfo.Outputs["message"])
			assert.Equal(t, "hello world", stackInfo.Outputs["nestedMessage"])
		},
	})
}

func testConstructComponentConfigureProviderCommonOptions() integration.ProgramTestOptions {
	const testDir = "construct_component_configure_provider"
	localProvider := integration.LocalDependency{
		Package: "metaprovider", Path: filepath.Join(testDir, "testcomponent-go"),
	}
	return integration.ProgramTestOptions{
		NoParallel: true,
		Config: map[string]string{
			"proxy": "FromEnv",
		},
		LocalProviders:           []integration.LocalDependency{localProvider},
		Quick:                    false, // intentional, need to test preview here
		AllowEmptyPreviewChanges: true,  // Pulumi will warn that provider has unknowns in its config
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.Contains(t, stackInfo.Outputs, "keyAlgo")
			assert.Equal(t, "ECDSA", stackInfo.Outputs["keyAlgo"])
			assert.Contains(t, stackInfo.Outputs, "keyAlgo2")
			assert.Equal(t, "ECDSA", stackInfo.Outputs["keyAlgo2"])

			var providerURNID string
			for _, r := range stackInfo.Deployment.Resources {
				if strings.Contains(string(r.URN), "PrivateKey") {
					providerURNID = r.Provider
				}
			}
			require.NotEmptyf(t, providerURNID, "Did not find the provider of PrivateKey resource")
			var providerFromEnvSetting *bool
			for _, r := range stackInfo.Deployment.Resources {
				if fmt.Sprintf("%s::%s", r.URN, r.ID) == providerURNID {
					providerFromEnvSetting = new(bool)

					proxy, ok := r.Inputs["proxy"]
					require.Truef(t, ok, "expected %q Inputs to contain 'proxy'", providerURNID)

					proxyMap, ok := proxy.(map[string]any)
					require.Truef(t, ok, "expected %q Inputs 'proxy' to be of type map[string]any", providerURNID)

					fromEnv, ok := proxyMap["fromEnv"]
					require.Truef(t, ok, "expected %q Inputs 'proxy' to contain 'fromEnv'", providerURNID)

					fromEnvB, ok := fromEnv.(bool)
					require.Truef(t, ok, "expected %q Inputs 'proxy.fromEnv' to have type bool", providerURNID)

					*providerFromEnvSetting = fromEnvB
				}
			}
			require.NotNilf(t, providerFromEnvSetting,
				"Did not find the inputs of the provider PrivateKey was provisioned with")
			require.Truef(t, *providerFromEnvSetting,
				"Expected PrivateKey to be provisioned with a provider with fromEnv=true")

			require.Equalf(t, float64(42), stackInfo.Outputs["meaningOfLife"],
				"Expected meaningOfLife output to be set to the integer 42")
			require.Equalf(t, float64(42), stackInfo.Outputs["meaningOfLife2"],
				"Expected meaningOfLife2 output to be set to the integer 42")
		},
	}
}

// Test failures returned from construct.
func testConstructFailures(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_failures"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-go",
		},
		{
			componentDir: "testcomponent-python",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			expectedError := `error: testcomponent:index:Component resource 'component' has a problem: failing for a reason
    		- property foo with value '{bar}' has a problem: the failure reason`

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

// Test failures returned from call.
func testCallFailures(t *testing.T, lang string, dependencies ...string) {
	const testDir = "call_component_failures"
	runComponentSetup(t, testDir)

	tests := []struct {
		componentDir string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-go",
		},
		{
			componentDir: "testcomponent-python",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.componentDir, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			expectedError := `error: call to function 'testcomponent:index:Component/getMessage' failed:
    		- property foo has a problem: the failure reason`

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
