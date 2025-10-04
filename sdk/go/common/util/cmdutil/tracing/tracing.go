// Copyright 2025, Pulumi Corporation.
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

package tracing

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	traceSDK "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const tracerName = "github.com/pulumi/pulumi/sdk/go/common/util/cmdutil/tracing"

// Init injects OTEL tracing into the returned context.
//
// If a nil error is returned, the closer must be closed *after* the last OTEL span is
// emitted.
//
// Valid arguments are:
// - http  (OTEL_EXPORTER_OTLP_ENDPOINT controls the OTLP endpoint)
// - https (OTEL_EXPORTER_OTLP_ENDPOINT controls the OTLP endpoint)
// - file:<path>
func Init(ctx context.Context, arg string) (context.Context, io.Closer, error) {
	var exporter traceSDK.SpanExporter
	var closeExporter io.Closer
	var err error
	if path, ok := strings.CutPrefix(arg, "file:"); ok {
		exporter, closeExporter, err = toFile(path)
	} else if path == "http" {
		exporter, closeExporter, err = toHTTP(ctx)
	} else if path == "https" {
		exporter, closeExporter, err = toHTTPS(ctx)
	} else {
		return nil, nil, errors.New("unknown tracing prefix")
	}

	if err != nil {
		return nil, nil, errors.Join(closeExporter.Close(), err)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("pulumi/pulumi"),
		),
	)
	if err != nil {
		return nil, nil, errors.Join(closeExporter.Close(), err)
	}

	tracerProvider := traceSDK.NewTracerProvider(
		traceSDK.WithBatcher(exporter),
		traceSDK.WithResource(res),
	)

	tracer := tracerProvider.Tracer(tracerName)
	ctx = context.WithValue(ctx, tracerKeyType{}, tracer)

	// Since this is used only for short-running CLI applications, we initialize a root span here.
	ctx, span := tracer.Start(ctx, "root")
	span.SetAttributes(attribute.String("service.name", "pulumi/pulumi"))

	return ctx, closeF(func() error {
		span.End()
		return errors.Join(
			tracerProvider.ForceFlush(ctx),
			tracerProvider.Shutdown(ctx),
			closeExporter.Close(),
		)
	}), nil
}

type tracerKeyType struct{}

var noopTraceProvider = noop.NewTracerProvider().Tracer(tracerName)

// Span creates a new span, inheriting from the passed in context.
func Span(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	v, ok := ctx.Value(tracerKeyType{}).(trace.Tracer)
	if !ok {
		return noopTraceProvider.Start(ctx, spanName, opts...)
	}
	return v.Start(ctx, spanName, opts...)
}

// See https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
// for the env vars that control pathing.
func toHTTP(ctx context.Context) (traceSDK.SpanExporter, io.Closer, error) {
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure())
	return exporter, closeF(func() error { return nil }), err
}

// See https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
// for the env vars that control pathing.
func toHTTPS(ctx context.Context) (traceSDK.SpanExporter, io.Closer, error) {
	exporter, err := otlptracehttp.New(ctx)
	return exporter, closeF(func() error { return nil }), err
}

func toFile(file string) (traceSDK.SpanExporter, io.Closer, error) {
	f, err := os.Create(file)
	if err != nil {
		return nil, nil, err
	}

	exporter, err := stdouttrace.New(stdouttrace.WithWriter(f))
	if err != nil {
		return nil, nil, errors.Join(err, f.Close())
	}
	return exporter, f, nil
}

type closeF func() error

func (f closeF) Close() error { return f() }
