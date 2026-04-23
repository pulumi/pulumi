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
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/appdash"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// AppDashBridge receives spans via the AppDash TCP protocol (used by legacy
// OpenTracing plugins) and converts them to otel ResourceSpans, forwarding
// them to a SpanExporter.
type AppDashBridge struct {
	server    *appdash.CollectorServer
	listener  *net.TCPListener
	port      int
	exporter  SpanExporter
	collector *bridgeCollector
}

// StartAppDashBridge creates and starts an AppDash TCP server that bridges
// legacy OpenTracing spans to otel.
func StartAppDashBridge(exporter SpanExporter) (*AppDashBridge, error) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		return nil, fmt.Errorf("failed to listen for AppDash bridge: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	collector := &bridgeCollector{
		exporter: exporter,
		pending:  make(map[appdash.SpanID][]appdash.Annotation),
	}

	server := appdash.NewServer(listener, collector)
	// Suppress the noisy Accept-error log loop on shutdown.
	server.Log = log.New(io.Discard, "", 0)

	b := &AppDashBridge{
		server:    server,
		listener:  listener,
		port:      port,
		exporter:  exporter,
		collector: collector,
	}

	go server.Start()

	logging.V(5).Infof("AppDash bridge started on port %d", port)

	return b, nil
}

// Endpoint returns the AppDash TCP endpoint in the format expected by
// legacy plugins (tcp://127.0.0.1:PORT).
func (b *AppDashBridge) Endpoint() string {
	return fmt.Sprintf("tcp://127.0.0.1:%d", b.port)
}

// SetTraceParent tells the bridge to remap all AppDash trace IDs to the
// given OTel trace ID, and to parent root AppDash spans (those without an
// AppDash parent) under the given OTel span ID.  This puts bridged spans
// into the same trace as the CLI's native OTel spans.
func (b *AppDashBridge) SetTraceParent(traceID [16]byte, spanID [8]byte) {
	b.collector.mu.Lock()
	defer b.collector.mu.Unlock()
	b.collector.otelTraceID = traceID
	b.collector.otelParentSpanID = spanID
}

func (b *AppDashBridge) Shutdown(_ context.Context) error {
	return b.listener.Close()
}

// bridgeCollector implements appdash.Collector. It accumulates annotations
// per SpanID across multiple Collect calls. The appdash_opentracing recorder
// makes several Collect calls for a single span (name, tags, timespan).
// The Timespan event (_schema:Timespan) is always sent last, so we use it
// as the trigger to flush the accumulated span to OTLP.
type bridgeCollector struct {
	mu               sync.Mutex
	exporter         SpanExporter
	pending          map[appdash.SpanID][]appdash.Annotation
	otelTraceID      [16]byte // if non-zero, replaces AppDash trace IDs
	otelParentSpanID [8]byte  // parent for root AppDash spans
}

func (c *bridgeCollector) Collect(id appdash.SpanID, anns ...appdash.Annotation) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pending[id] = append(c.pending[id], anns...)

	// Check if this batch contains the Timespan schema marker, which
	// indicates the recorder has finished sending all annotations for
	// this span.
	hasTimespan := false
	for _, ann := range anns {
		if ann.Key == "_schema:Timespan" {
			hasTimespan = true
			break
		}
	}

	if !hasTimespan {
		return nil
	}

	allAnns := c.pending[id]
	delete(c.pending, id)
	otelTraceID := c.otelTraceID
	otelParentSpanID := c.otelParentSpanID

	resourceSpans := convertAppDashToOTLP(id, allAnns, otelTraceID, otelParentSpanID)
	if resourceSpans == nil {
		return nil
	}

	if err := c.exporter.ExportSpans(context.Background(), []*tracepb.ResourceSpans{resourceSpans}); err != nil {
		logging.V(5).Infof("AppDash bridge: failed to export span: %v", err)
		return err
	}

	return nil
}

// inferServiceName guesses a service name from the span name.  gRPC server
// interceptor spans look like "/pulumirpc.ResourceProvider/Check" while
// Pulumi-framework spans look like "pf.Configure".
func inferServiceName(spanName string) string {
	switch {
	case strings.Contains(spanName, "ResourceProvider"):
		return "pulumi-resource-provider"
	case strings.HasPrefix(spanName, "pf."):
		return "pulumi-resource-provider"
	case strings.Contains(spanName, "LanguageRuntime"):
		return "pulumi-language-host"
	case strings.Contains(spanName, "ResourceMonitor"),
		strings.Contains(spanName, "Engine/"):
		// Calls made by the language host to the engine/monitor
		return "pulumi-language-host"
	default:
		return "opentracing-plugin"
	}
}

// convertAppDashToOTLP converts an AppDash span (identified by SpanID and
// accumulated annotations) into OTLP ResourceSpans.  If otelTraceID is
// non-zero the AppDash trace ID is replaced so that bridged spans appear in
// the same trace as the CLI's native OTel spans.  Root AppDash spans (no
// AppDash parent) are parented under otelParentSpanID.
func convertAppDashToOTLP(
	id appdash.SpanID, anns []appdash.Annotation,
	otelTraceID [16]byte, otelParentSpanID [8]byte,
) *tracepb.ResourceSpans {
	var name string
	var serviceName string
	var startTime, endTime time.Time
	var hasError bool
	var attributes []*commonpb.KeyValue

	for _, ann := range anns {
		switch {
		case ann.Key == "Name":
			name = string(ann.Value)
		case ann.Key == "ServerName":
			serviceName = string(ann.Value)
		case ann.Key == "Span.Start":
			if t, err := time.Parse(time.RFC3339Nano, string(ann.Value)); err == nil {
				startTime = t
			}
		case ann.Key == "Span.End":
			if t, err := time.Parse(time.RFC3339Nano, string(ann.Value)); err == nil {
				endTime = t
			}
		case ann.Key == "error":
			if string(ann.Value) == "true" {
				hasError = true
			}
		case strings.HasPrefix(ann.Key, "_schema:"):
			// Schema markers — skip.
		default:
			attributes = append(attributes, &commonpb.KeyValue{
				Key: ann.Key,
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{StringValue: string(ann.Value)},
				},
			})
		}
	}

	if name == "" {
		name = fmt.Sprintf("span-%d", id.Span)
	}

	// Build trace ID.  If an OTel trace ID override is set, use it so that
	// bridged spans land in the same trace as native OTel spans.  Otherwise
	// map the AppDash uint64 trace ID into the low 8 bytes of a 16-byte ID.
	var zeroTraceID [16]byte
	var traceID [16]byte
	if otelTraceID != zeroTraceID {
		traceID = otelTraceID
	} else {
		binary.BigEndian.PutUint64(traceID[8:], uint64(id.Trace))
	}

	var spanID [8]byte
	binary.BigEndian.PutUint64(spanID[:], uint64(id.Span))

	span := &tracepb.Span{
		TraceId:    traceID[:],
		SpanId:     spanID[:],
		Name:       name,
		Attributes: attributes,
	}

	// Set parent span ID.  If the AppDash span has a parent, keep it.
	// Otherwise, if an OTel parent override is set, use it to graft the
	// root of the AppDash tree onto the OTel trace.
	var zeroSpanID [8]byte
	if id.Parent != 0 {
		var parentSpanID [8]byte
		binary.BigEndian.PutUint64(parentSpanID[:], uint64(id.Parent))
		span.ParentSpanId = parentSpanID[:]
	} else if otelParentSpanID != zeroSpanID {
		span.ParentSpanId = otelParentSpanID[:]
	}

	if hasError {
		span.Status = &tracepb.Status{
			Code: tracepb.Status_STATUS_CODE_ERROR,
		}
	}

	if !startTime.IsZero() {
		span.StartTimeUnixNano = uint64(startTime.UnixNano()) //nolint:gosec // timestamps are always positive
	}
	if !endTime.IsZero() {
		span.EndTimeUnixNano = uint64(endTime.UnixNano()) //nolint:gosec // timestamps are always positive
	}

	if serviceName == "" {
		serviceName = inferServiceName(name)
	}

	return &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key: "service.name",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: serviceName},
					},
				},
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{span},
			},
		},
	}
}
