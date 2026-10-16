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

package otelreceiver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestFileExporter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "traces.json")

	exporter, err := newFileExporter(path)
	require.NoError(t, err)

	makeSpan := func(name string) []*tracepb.ResourceSpans {
		return []*tracepb.ResourceSpans{
			{
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{{Name: name}},
					},
				},
			},
		}
	}

	err = exporter.ExportSpans(t.Context(), makeSpan("span1"))
	require.NoError(t, err)

	err = exporter.ExportSpans(t.Context(), makeSpan("span2"))
	require.NoError(t, err)

	err = exporter.Shutdown(t.Context())
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	require.Equal(t,
		`{"resourceSpans":[{"resource":{},"scopeSpans":[{"scope":{},"spans":[{"name":"span1","status":{}}]}]}]}
{"resourceSpans":[{"resource":{},"scopeSpans":[{"scope":{},"spans":[{"name":"span2","status":{}}]}]}]}
`, string(data))
}
