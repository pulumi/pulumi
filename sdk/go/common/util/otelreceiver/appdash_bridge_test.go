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
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/appdash"
	appdash_opentracing "github.com/pulumi/appdash/opentracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestConvertAppDashToOTLP(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Nanosecond)
	end := now.Add(100 * time.Millisecond)

	id := appdash.SpanID{
		Trace:  appdash.ID(12345),
		Span:   appdash.ID(67890),
		Parent: appdash.ID(11111),
	}

	anns := []appdash.Annotation{
		{Key: "Name", Value: []byte("test-span")},
		{Key: "ServerName", Value: []byte("test-service")},
		{Key: "Span.Start", Value: []byte(now.Format(time.RFC3339Nano))},
		{Key: "Span.End", Value: []byte(end.Format(time.RFC3339Nano))},
		{Key: "_schema:Timespan", Value: nil},
		{Key: "http.method", Value: []byte("GET")},
		{Key: "http.status_code", Value: []byte("200")},
	}

	result := convertAppDashToOTLP(id, anns, [16]byte{}, [8]byte{})
	require.NotNil(t, result)

	require.NotNil(t, result.Resource)
	require.Len(t, result.Resource.Attributes, 1)
	assert.Equal(t, "service.name", result.Resource.Attributes[0].Key)
	assert.Equal(t, "test-service", result.Resource.Attributes[0].Value.GetStringValue())

	require.Len(t, result.ScopeSpans, 1)
	require.Len(t, result.ScopeSpans[0].Spans, 1)
	span := result.ScopeSpans[0].Spans[0]

	assert.Equal(t, "test-span", span.Name)

	traceIDLow := binary.BigEndian.Uint64(span.TraceId[8:])
	assert.Equal(t, uint64(12345), traceIDLow)

	spanID := binary.BigEndian.Uint64(span.SpanId)
	assert.Equal(t, uint64(67890), spanID)

	parentID := binary.BigEndian.Uint64(span.ParentSpanId)
	assert.Equal(t, uint64(11111), parentID)

	assert.Equal(t, uint64(now.UnixNano()), span.StartTimeUnixNano) //nolint:gosec // test timestamps
	assert.Equal(t, uint64(end.UnixNano()), span.EndTimeUnixNano)   //nolint:gosec // test timestamps

	require.Len(t, span.Attributes, 2)
	attrMap := map[string]string{}
	for _, attr := range span.Attributes {
		attrMap[attr.Key] = attr.Value.GetStringValue()
	}
	assert.Equal(t, "GET", attrMap["http.method"])
	assert.Equal(t, "200", attrMap["http.status_code"])
}

func TestConvertAppDashToOTLP_NoParent(t *testing.T) {
	t.Parallel()

	id := appdash.SpanID{
		Trace:  appdash.ID(99999),
		Span:   appdash.ID(11111),
		Parent: 0,
	}

	anns := []appdash.Annotation{
		{Key: "Name", Value: []byte("root-span")},
	}

	result := convertAppDashToOTLP(id, anns, [16]byte{}, [8]byte{})
	require.NotNil(t, result)

	span := result.ScopeSpans[0].Spans[0]
	assert.Equal(t, "root-span", span.Name)
	assert.Empty(t, span.ParentSpanId)

	require.Len(t, result.Resource.Attributes, 1)
	assert.Equal(t, "service.name", result.Resource.Attributes[0].Key)
	assert.Equal(t, "opentracing-plugin", result.Resource.Attributes[0].Value.GetStringValue())
}

func TestConvertAppDashToOTLP_NoName(t *testing.T) {
	t.Parallel()

	id := appdash.SpanID{
		Trace: appdash.ID(1),
		Span:  appdash.ID(42),
	}

	result := convertAppDashToOTLP(id, nil, [16]byte{}, [8]byte{})
	require.NotNil(t, result)

	span := result.ScopeSpans[0].Spans[0]
	assert.Equal(t, "span-42", span.Name)

	require.Len(t, result.Resource.Attributes, 1)
	assert.Equal(t, "opentracing-plugin", result.Resource.Attributes[0].Value.GetStringValue())
}

func TestConvertAppDashToOTLP_TraceOverride(t *testing.T) {
	t.Parallel()

	id := appdash.SpanID{
		Trace:  appdash.ID(12345),
		Span:   appdash.ID(67890),
		Parent: 0, // root span
	}

	anns := []appdash.Annotation{
		{Key: "Name", Value: []byte("provider-op")},
	}

	otelTraceID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	otelParentSpanID := [8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22}

	result := convertAppDashToOTLP(id, anns, otelTraceID, otelParentSpanID)
	require.NotNil(t, result)

	span := result.ScopeSpans[0].Spans[0]

	assert.Equal(t, otelTraceID[:], span.TraceId)

	spanIDVal := binary.BigEndian.Uint64(span.SpanId)
	assert.Equal(t, uint64(67890), spanIDVal)

	assert.Equal(t, otelParentSpanID[:], span.ParentSpanId)
}

func TestConvertAppDashToOTLP_TraceOverrideKeepsAppDashParent(t *testing.T) {
	t.Parallel()

	id := appdash.SpanID{
		Trace:  appdash.ID(12345),
		Span:   appdash.ID(67890),
		Parent: appdash.ID(11111), // has AppDash parent
	}

	anns := []appdash.Annotation{
		{Key: "Name", Value: []byte("child-op")},
	}

	otelTraceID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	otelParentSpanID := [8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22}

	result := convertAppDashToOTLP(id, anns, otelTraceID, otelParentSpanID)
	require.NotNil(t, result)

	span := result.ScopeSpans[0].Spans[0]

	assert.Equal(t, otelTraceID[:], span.TraceId)

	parentID := binary.BigEndian.Uint64(span.ParentSpanId)
	assert.Equal(t, uint64(11111), parentID)
}

func TestInferServiceName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pulumi-resource-provider", inferServiceName("/pulumirpc.ResourceProvider/Check"))
	assert.Equal(t, "pulumi-resource-provider", inferServiceName("pf.Configure"))
	assert.Equal(t, "pulumi-language-host", inferServiceName("/pulumirpc.LanguageRuntime/Run"))
	assert.Equal(t, "pulumi-language-host", inferServiceName("/pulumirpc.ResourceMonitor/SupportsFeature"))
	assert.Equal(t, "pulumi-language-host", inferServiceName("/pulumirpc.Engine/Log"))
	assert.Equal(t, "opentracing-plugin", inferServiceName("some-custom-span"))
}

func TestAppDashBridgeEndToEnd(t *testing.T) {
	t.Parallel()

	exporter := &mockExporter{}

	bridge, err := StartAppDashBridge(exporter)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_ = bridge.Shutdown(ctx)
	}()

	endpoint := bridge.Endpoint()
	assert.True(t, strings.HasPrefix(endpoint, "tcp://127.0.0.1:"))

	hostPort := strings.TrimPrefix(endpoint, "tcp://")
	remoteCollector := appdash.NewRemoteCollector(hostPort)

	tracer := appdash_opentracing.NewTracer(remoteCollector)

	span := tracer.StartSpan("test-operation")
	span.SetTag("test.key", "test-value")
	span.Finish()

	err = remoteCollector.Close()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return exporter.spanCount() > 0
	}, 5*time.Second, 50*time.Millisecond, "expected spans to be exported")

	snaps := exporter.snapshot()
	require.GreaterOrEqual(t, len(snaps), 1)

	var found bool
	for _, rs := range snaps {
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				if s.Name == "test-operation" {
					found = true
					assert.NotEqual(t, make([]byte, 16), s.TraceId)
					assert.NotEqual(t, make([]byte, 8), s.SpanId)
					assert.NotZero(t, s.StartTimeUnixNano)
					assert.NotZero(t, s.EndTimeUnixNano)
				}
			}
		}
	}
	assert.True(t, found, "expected to find span named 'test-operation'")
}

func TestAppDashBridgeParentChild(t *testing.T) {
	t.Parallel()

	exporter := &mockExporter{}

	bridge, err := StartAppDashBridge(exporter)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_ = bridge.Shutdown(ctx)
	}()

	hostPort := strings.TrimPrefix(bridge.Endpoint(), "tcp://")
	remoteCollector := appdash.NewRemoteCollector(hostPort)

	tracer := appdash_opentracing.NewTracer(remoteCollector)

	parent := tracer.StartSpan("parent-op")
	child := tracer.StartSpan("child-op", opentracing.ChildOf(parent.Context()))
	child.Finish()
	parent.Finish()

	err = remoteCollector.Close()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return exporter.spanCount() >= 2
	}, 5*time.Second, 50*time.Millisecond, "expected at least 2 spans")

	snaps := exporter.snapshot()
	var childSpan *tracepb.Span
	var parentSpan *tracepb.Span
	for _, rs := range snaps {
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				switch s.Name {
				case "child-op":
					childSpan = s
				case "parent-op":
					parentSpan = s
				}
			}
		}
	}

	require.NotNil(t, childSpan, "expected child span")
	require.NotNil(t, parentSpan, "expected parent span")

	assert.Equal(t, parentSpan.SpanId, childSpan.ParentSpanId)

	assert.Equal(t, parentSpan.TraceId, childSpan.TraceId)
}
