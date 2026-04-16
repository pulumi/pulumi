// Copyright 2016, Pulumi Corporation.
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
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// mockExporter is a test exporter that records spans for verification.
// All access is protected by a mutex so the race detector stays happy
// when the bridge goroutine writes while test goroutines poll.
type mockExporter struct {
	mu    sync.Mutex
	spans []*tracepb.ResourceSpans
}

func (m *mockExporter) ExportSpans(_ context.Context, spans []*tracepb.ResourceSpans) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spans = append(m.spans, spans...)
	return nil
}

func (m *mockExporter) Shutdown(_ context.Context) error {
	return nil
}

// snapshot returns a copy of the collected spans under the lock.
func (m *mockExporter) snapshot() []*tracepb.ResourceSpans {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*tracepb.ResourceSpans, len(m.spans))
	copy(cp, m.spans)
	return cp
}

// spanCount returns the total number of individual spans collected.
func (m *mockExporter) spanCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, rs := range m.spans {
		for _, ss := range rs.ScopeSpans {
			n += len(ss.Spans)
		}
	}
	return n
}

func TestReceiverWithExporter(t *testing.T) {
	t.Parallel()

	// Create a mock exporter to capture spans
	exporter := &mockExporter{}

	receiver, err := Start(exporter)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_ = receiver.Shutdown(ctx)
	}()

	conn, err := grpc.NewClient(
		receiver.Endpoint(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := coltracepb.NewTraceServiceClient(conn)
	ctx := t.Context()

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
								SpanId:  []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Name:    "forwarded-span",
							},
						},
					},
				},
			},
		},
	}

	resp, err := client.Export(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	snaps := exporter.snapshot()
	require.Len(t, snaps, 1)
	assert.Equal(t, "forwarded-span", snaps[0].ScopeSpans[0].Spans[0].Name)
}
