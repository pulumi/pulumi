// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

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
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
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

//nolint:deadcode
func pathEnv(t *testing.T, path ...string) string {
	pathEnv := []string{os.Getenv("PATH")}
	for _, p := range path {
		absPath, err := filepath.Abs(p)
		if err != nil {
			t.Fatal(err)
			return ""
		}
		pathEnv = append(pathEnv, absPath)
	}
	pathSeparator := ":"
	if runtime.GOOS == "windows" {
		pathSeparator = ";"
	}
	return "PATH=" + strings.Join(pathEnv, pathSeparator)
}

//nolint:deadcode
func testComponentSlowPathEnv(t *testing.T) string {
	return pathEnv(t, filepath.Join("construct_component_slow", "testcomponent"))
}

//nolint:deadcode
func testComponentPlainPathEnv(t *testing.T) string {
	return pathEnv(t, filepath.Join("construct_component_plain", "testcomponent"))
}

// nolint: unused,deadcode
func testComponentProviderSchema(t *testing.T, path string) {
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
		t.Run(test.name, func(t *testing.T) {
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
// nolint: unused,deadcode
func testConstructUnknown(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_unknown"
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
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t,
				filepath.Join("..", "testprovider"),
				filepath.Join(testDir, test.componentDir))
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Env:                    []string{pathEnv},
				Dir:                    filepath.Join(testDir, lang),
				Dependencies:           dependencies,
				SkipRefresh:            true,
				SkipPreview:            false,
				SkipUpdate:             true,
				SkipExportImport:       true,
				SkipEmptyPreviewUpdate: true,
				Quick:                  false,
				NoParallel:             true,
			})
		})
	}
}

// Test methods properly handle unknowns.
// nolint: unused,deadcode
func testConstructMethodsUnknown(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_methods_unknown"
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
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t,
				filepath.Join("..", "testprovider"),
				filepath.Join(testDir, test.componentDir))
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Env:                    []string{pathEnv},
				Dir:                    filepath.Join(testDir, lang),
				Dependencies:           dependencies,
				SkipRefresh:            true,
				SkipPreview:            false,
				SkipUpdate:             true,
				SkipExportImport:       true,
				SkipEmptyPreviewUpdate: true,
				Quick:                  false,
				NoParallel:             true,
			})
		})
	}
}

// Test methods that create resources.
// nolint: unused,deadcode
func testConstructMethodsResources(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_methods_resources"
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
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t,
				filepath.Join("..", "testprovider"),
				filepath.Join(testDir, test.componentDir))
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Env:          []string{pathEnv},
				Dir:          filepath.Join(testDir, lang),
				Dependencies: dependencies,
				Quick:        true,
				NoParallel:   true, // avoid contention for Dir
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
// nolint: unused,deadcode
func testConstructMethodsErrors(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_methods_errors"
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
		t.Run(test.componentDir, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			expectedError := "the failure reason (the failure property)"

			pathEnv := pathEnv(t, filepath.Join(testDir, test.componentDir))
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Env:           []string{pathEnv},
				Dir:           filepath.Join(testDir, lang),
				Dependencies:  dependencies,
				Quick:         true,
				NoParallel:    true, // avoid contention for Dir
				Stderr:        stderr,
				ExpectFailure: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					output := stderr.String()
					assert.Contains(t, output, expectedError)
				},
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

// nolint: unused,deadcode
func testConstructOutputValues(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_output_values"
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
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t,
				filepath.Join("..", "testprovider"),
				filepath.Join(testDir, test.componentDir))
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Env:          []string{pathEnv},
				Dir:          filepath.Join(testDir, lang),
				Dependencies: dependencies,
				Quick:        true,
				NoParallel:   true, // avoid contention for Dir
			})
		})
	}
}

// printfTestValidation is used by the TestPrintfXYZ test cases in the language-specific test
// files. It validates that there are a precise count of expected stdout/stderr lines in the test output.
//nolint:deadcode // The linter doesn't see the uses since the consumers are conditionally compiled tests.
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
