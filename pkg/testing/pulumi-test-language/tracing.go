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

package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "pulumi-test-language"

var tracer = otel.Tracer(tracerName)

// fileSpanExporter writes completed spans as JSON lines to a file.
type fileSpanExporter struct {
	mu   sync.Mutex
	file *os.File
}

type spanRecord struct {
	Name       string            `json:"name"`
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	StartTime  int64             `json:"start_ms"`
	EndTime    int64             `json:"end_ms"`
	DurationMs int64             `json:"duration_ms"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Status     string            `json:"status,omitempty"`
}

func newFileSpanExporter(path string) (*fileSpanExporter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	return &fileSpanExporter{file: f}, nil
}

func (e *fileSpanExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	enc := json.NewEncoder(e.file)
	for _, s := range spans {
		attrs := map[string]string{}
		for _, a := range s.Attributes() {
			attrs[string(a.Key)] = a.Value.Emit()
		}

		rec := spanRecord{
			Name:       s.Name(),
			TraceID:    s.SpanContext().TraceID().String(),
			SpanID:     s.SpanContext().SpanID().String(),
			StartTime:  s.StartTime().UnixMilli(),
			EndTime:    s.EndTime().UnixMilli(),
			DurationMs: s.EndTime().Sub(s.StartTime()).Milliseconds(),
			Attributes: attrs,
			Status:     s.Status().Code.String(),
		}
		if s.Parent().IsValid() {
			rec.ParentID = s.Parent().SpanID().String()
		}

		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}

func (e *fileSpanExporter) Shutdown(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.file.Close()
}

// initTracing initializes OpenTelemetry tracing.
// If PULUMI_TEST_TRACE_FILE is set, writes spans to that file as JSON lines.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, sends spans via gRPC.
// Returns a shutdown function that should be called when tracing is no longer needed.
func initTracing(ctx context.Context) func(context.Context) {
	var exporter sdktrace.SpanExporter

	// Check for file-based exporter first (simpler, no collector needed)
	if traceFile := os.Getenv("PULUMI_TEST_TRACE_FILE"); traceFile != "" {
		var err error
		exporter, err = newFileSpanExporter(traceFile)
		if err != nil {
			return func(context.Context) {}
		}
	} else {
		// Fall back to gRPC OTLP exporter
		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if endpoint == "" {
			endpoint = os.Getenv("PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT")
		}
		if endpoint == "" {
			return func(context.Context) {}
		}

		var err error
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(endpoint),
		)
		if err != nil {
			return func(context.Context) {}
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(tracerName),
		)),
	)

	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(tracerName)

	return func(ctx context.Context) {
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	}
}

// startSpan creates a new span with the given name and attributes.
func startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}
