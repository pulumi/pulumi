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

package ints

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

// TestOtelTraces runs a basic `pulumi up` with --otel-traces and asserts that the captured trace
// has exactly one root span. A well-formed trace should be a single tree rooted at the CLI's
// "pulumi" span; multiple roots mean a span was emitted somewhere without inheriting the parent
// context — typically a goroutine or RPC handler that wasn't wired into the trace context
// propagation. We deliberately don't assert on the shape of the tree below the root, since the
// names and arrangement of spans under "pulumi" are an internal detail that's expected to evolve.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestOtelTraces(t *testing.T) {
	// Build the testprovider as a standalone binary so the engine launches it directly in ExecPlugin rather than
	// through RunPlugin. We currently can't gracefully shut down a RunPlugin process and wait for it to finish, so
	// providers launched that way don't get a chance to flush their OTEL traces before being killed.
	binDir := t.TempDir()
	binaryName := "pulumi-resource-testprovider"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(binDir, binaryName), ".") //nolint:gosec
	buildCmd.Dir = filepath.Join("..", "..", "testprovider")
	out, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "failed to build testprovider: %s", out)

	traceDir := t.TempDir()
	tracePath, err := filepath.Abs(filepath.Join(traceDir, "traces-{command}.json"))
	require.NoError(t, err)

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: "python",
		Dependencies: []string{
			filepath.Join("..", "..", "..", "sdk", "python"),
		},
		Env: []string{
			fmt.Sprintf("PATH=%s%c%s", binDir, os.PathListSeparator, os.Getenv("PATH")),
		},
		OtelTraces: "file:///" + tracePath,
		Quick:      true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			upTracePath := filepath.Join(traceDir, "traces-pulumi-update-initial.json")
			traces := readTraces(t, upTracePath)

			var rootDescriptions []string
			for i := range traces.ResourceSpans().Len() {
				rs := traces.ResourceSpans().At(i)
				serviceName := ""
				if v, ok := rs.Resource().Attributes().Get("service.name"); ok {
					serviceName = v.Str()
				}
				for j := range rs.ScopeSpans().Len() {
					spans := rs.ScopeSpans().At(j).Spans()
					for k := range spans.Len() {
						s := spans.At(k)
						if s.ParentSpanID().IsEmpty() {
							rootDescriptions = append(rootDescriptions, fmt.Sprintf(
								"  - name=%q service=%q span=%s trace=%s",
								s.Name(), serviceName, s.SpanID(), s.TraceID()))
						}
					}
				}
			}

			if len(rootDescriptions) != 1 {
				sort.Strings(rootDescriptions)
				assert.Fail(t, fmt.Sprintf("expected exactly 1 root span, got %d:\n%s",
					len(rootDescriptions), strings.Join(rootDescriptions, "\n")))
			}
		},
	})
}

// readTraces reads a newline-delimited OTLP JSON trace file (the format written by the file://
// exporter — each line is a separate ExportTraceServiceRequest) and merges every line into a
// single ptrace.Traces value.
func readTraces(t *testing.T, path string) ptrace.Traces {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err, "opening trace file %s", path)
	defer f.Close()

	var unmarshaler ptrace.JSONUnmarshaler
	out := ptrace.NewTraces()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		traces, err := unmarshaler.UnmarshalTraces(line)
		require.NoError(t, err, "unmarshaling trace line")
		traces.ResourceSpans().MoveAndAppendTo(out.ResourceSpans())
	}
	require.NoError(t, scanner.Err(), "scanning trace file")
	require.NotZero(t, out.SpanCount(), "expected at least one span in trace file %s", path)

	return out
}
