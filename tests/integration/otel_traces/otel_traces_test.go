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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

type traceSpan struct {
	Name        string
	SpanID      pcommon.SpanID
	ParentID    pcommon.SpanID
	TraceID     pcommon.TraceID
	ServiceName string
	Attributes  map[string]string
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestOtelTraces(t *testing.T) {
	traceDir := t.TempDir()
	tracePath := filepath.Join(traceDir, "traces-{command}.json")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: "python",
		Dependencies: []string{
			filepath.Join("..", "..", "..", "sdk", "python"),
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		OtelTraces: "file://" + tracePath,
		Quick:      true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			upTracePath := filepath.Join(traceDir, "traces-pulumi-update-initial.json")
			spans := readSpans(t, upTracePath)

			spanByID := make(map[pcommon.SpanID]traceSpan)
			for _, s := range spans {
				spanByID[s.SpanID] = s
			}

			// Walk through the expected span hierarchy top-down:
			//
			//   pulumi                                    (pulumi-cli)
			//   └── pulumi-plan                           (pulumi-cli)
			//       ├── LanguageRuntime/Run               (pulumi-language-python)
			//       │   └── RegisterResource              (pulumi-sdk-python, client)
			//       │       └── RegisterResource          (pulumi-cli, server)
			//       └── ResourceProvider/Create           (pulumi-cli, client)
			//           └── ResourceProvider/Create       (testprovider, server)

			// Root pulumi span: the CLI entry point, no parent.
			rootSpans := findSpans(spans, "pulumi")
			require.NotEmpty(t, rootSpans, "expected root 'pulumi' span")
			assert.Equal(t, "pulumi-cli", rootSpans[0].ServiceName)
			assert.True(t, rootSpans[0].ParentID.IsEmpty())
			rootAttrs := rootSpans[0].Attributes
			assert.Equal(t, rootAttrs["cli.command"], "pulumi up")
			assert.NotEmpty(t, rootAttrs["pulumi.version"])
			assert.NotEmpty(t, rootAttrs["pulumi.os"])
			assert.NotEmpty(t, rootAttrs["pulumi.arch"])
			assert.NotEmpty(t, rootAttrs["exec.kind"])

			// pulumi-plan: the engine's deployment span, child of root.
			planSpans := findSpans(spans, "pulumi-plan")
			require.NotEmpty(t, planSpans)
			assert.Equal(t, "pulumi-cli", planSpans[0].ServiceName)
			assert.True(t, isDescendantOf(planSpans[0], rootSpans[0], spanByID))

			// LanguageRuntime/Run: the language host handling the Run RPC from the engine.
			pyRunSpans := filterByService(findSpans(spans, "pulumirpc.LanguageRuntime/Run"), "pulumi-language-python")
			require.NotEmpty(t, pyRunSpans)

			// RegisterResource (client side): the Python SDK calling the engine to register a resource, descendant of
			// LanguageRuntime/Run.
			// Python SDK client spans have a leading "/" in gRPC method names.
			pyRegSpans := filterByService(findSpans(spans, "/pulumirpc.ResourceMonitor/RegisterResource"), "pulumi-sdk-python")
			require.NotEmpty(t, pyRegSpans)
			for _, s := range pyRegSpans {
				assert.True(t, isDescendantOf(s, pyRunSpans[0], spanByID))
			}

			// RegisterResource (server side): the engine handling the RegisterResource call. Each engine-side span
			// should be a child of a Python SDK client-side span.
			engineRegSpans := filterByService(
				findSpans(spans, "pulumirpc.ResourceMonitor/RegisterResource"), "pulumi-cli")
			require.NotEmpty(t, engineRegSpans)
			for _, s := range engineRegSpans {
				assert.True(t, isDescendantOfAny(s, pyRegSpans, spanByID))
			}

			// ResourceProvider/Create: the engine calling the provider to create the resource, descendant of
			// pulumi-plan.
			createSpans := filterByService(findSpans(spans, "pulumirpc.ResourceProvider/Create"), "pulumi-cli")
			require.NotEmpty(t, createSpans, "expected provider Create span")
			for _, s := range createSpans {
				assert.True(t, isDescendantOf(s, planSpans[0], spanByID))
			}

			// ResourceProvider/Create (server side): the provider handling the Create RPC. These spans come from the
			// testprovider process itself.
			providerCreateSpans := filterByService(findSpans(spans, "pulumirpc.ResourceProvider/Create"), "testprovider")
			require.NotEmpty(t, providerCreateSpans)
			for _, s := range providerCreateSpans {
				assert.True(t, isDescendantOfAny(s, createSpans, spanByID))
			}
		},
	})
}

// findSpans returns all spans whose name exactly matches the given string.
func findSpans(spans []traceSpan, name string) []traceSpan {
	var result []traceSpan
	for _, s := range spans {
		if s.Name == name {
			result = append(result, s)
		}
	}
	return result
}

// filterByService returns spans with the given service name.
func filterByService(spans []traceSpan, service string) []traceSpan {
	var result []traceSpan
	for _, s := range spans {
		if s.ServiceName == service {
			result = append(result, s)
		}
	}
	return result
}

// isDescendantOfAny returns true if span is a descendant of any of the given ancestors.
func isDescendantOfAny(span traceSpan, ancestors []traceSpan, index map[pcommon.SpanID]traceSpan) bool {
	for _, a := range ancestors {
		if isDescendantOf(span, a, index) {
			return true
		}
	}
	return false
}

// isDescendantOf walks the parent chain of span and returns true if it reaches ancestor.
func isDescendantOf(span, ancestor traceSpan, index map[pcommon.SpanID]traceSpan) bool {
	current := span
	seen := make(map[pcommon.SpanID]bool)
	for {
		if current.SpanID == ancestor.SpanID {
			return true
		}
		if current.ParentID.IsEmpty() {
			return false
		}
		if seen[current.ParentID] {
			return false
		}
		seen[current.ParentID] = true
		parent, ok := index[current.ParentID]
		if !ok {
			return false
		}
		current = parent
	}
}

// readSpans reads a newline-delimited OTLP JSON trace file and returns all spans with their metadata.
func readSpans(t *testing.T, path string) []traceSpan {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err, "opening trace file %s", path)
	defer f.Close()

	var unmarshaler ptrace.JSONUnmarshaler
	var spans []traceSpan

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		traces, err := unmarshaler.UnmarshalTraces(line)
		require.NoError(t, err, "unmarshaling trace line")

		for i := range traces.ResourceSpans().Len() {
			rs := traces.ResourceSpans().At(i)

			serviceName := ""
			if val, ok := rs.Resource().Attributes().Get("service.name"); ok {
				serviceName = val.Str()
			}

			for j := range rs.ScopeSpans().Len() {
				ss := rs.ScopeSpans().At(j)
				for k := range ss.Spans().Len() {
					s := ss.Spans().At(k)
					attrs := make(map[string]string)
					s.Attributes().Range(func(k string, v pcommon.Value) bool {
						attrs[k] = v.AsString()
						return true
					})
					spans = append(spans, traceSpan{
						Name:        s.Name(),
						SpanID:      s.SpanID(),
						ParentID:    s.ParentSpanID(),
						TraceID:     s.TraceID(),
						ServiceName: serviceName,
						Attributes:  attrs,
					})
				}
			}
		}
	}
	require.NoError(t, scanner.Err(), "scanning trace file")
	require.NotEmpty(t, spans, "expected at least one span in trace file %s", path)

	return spans
}
