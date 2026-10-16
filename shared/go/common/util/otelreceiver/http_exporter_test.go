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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func testSpans() []*tracepb.ResourceSpans {
	return []*tracepb.ResourceSpans{
		{
			ScopeSpans: []*tracepb.ScopeSpans{
				{
					Spans: []*tracepb.Span{
						{
							Name:    "test-span",
							TraceId: []byte("0123456789abcdef"),
							SpanId:  []byte("01234567"),
						},
					},
				},
			},
		},
	}
}

func TestHTTPExporterExportSpans(t *testing.T) {
	t.Parallel()

	type received struct {
		method      string
		path        string
		contentType string
		apiKey      string
		body        []byte
	}
	requests := make(chan received, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requests <- received{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			apiKey:      r.Header.Get("X-Api-Key"),
			body:        body,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp, err := NewExporter(server.URL + "?x-api-key=testkey")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exp.Shutdown(t.Context()))
	}()

	require.NoError(t, exp.ExportSpans(t.Context(), testSpans()))

	req := <-requests
	require.Equal(t, http.MethodPost, req.method)
	require.Equal(t, "/v1/traces", req.path)
	require.Equal(t, "application/x-protobuf", req.contentType)
	require.Equal(t, "testkey", req.apiKey)

	var exportReq coltracepb.ExportTraceServiceRequest
	require.NoError(t, proto.Unmarshal(req.body, &exportReq))
	require.Len(t, exportReq.ResourceSpans, 1)
	require.Equal(t, "test-span", exportReq.ResourceSpans[0].ScopeSpans[0].Spans[0].Name)
}

func TestHTTPExporterEmptySpans(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected for empty spans")
	}))
	defer server.Close()

	exp, err := NewExporter(server.URL)
	require.NoError(t, err)
	require.NoError(t, exp.ExportSpans(t.Context(), nil))
	require.NoError(t, exp.Shutdown(t.Context()))
}

func TestHTTPExporterServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid api key", http.StatusUnauthorized)
	}))
	defer server.Close()

	exp, err := NewExporter(server.URL)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exp.Shutdown(t.Context()))
	}()

	err = exp.ExportSpans(t.Context(), testSpans())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "invalid api key")
}
