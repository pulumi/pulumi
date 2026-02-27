// Copyright 2016-2022, Pulumi Corporation.
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

package rpcutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Configures interceptors to propagate OpenTracing metadata through headers. If parentSpan is non-nil, it becomes the
// default parent for orphan spans.
func OpenTracingServerInterceptorOptions(parentSpan opentracing.Span, options ...otgrpc.Option) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(OpenTracingServerInterceptor(parentSpan, options...)),
		grpc.ChainStreamInterceptor(OpenTracingStreamServerInterceptor(parentSpan, options...)),
	}
}

// Configures interceptors to propagate OpenTracing and OpenTelemetry metadata through headers.
// If parentSpan is non-nil, it becomes the default parent for orphan spans.
func TracingServerInterceptorOptions(parentSpan opentracing.Span, options ...otgrpc.Option) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			otelUnaryServerInterceptor(),
			OpenTracingServerInterceptor(parentSpan, options...),
			stackTraceUnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			otelStreamServerInterceptor(),
			OpenTracingStreamServerInterceptor(parentSpan, options...),
			stackTraceStreamServerInterceptor(),
		),
	}
}

// Configures gRPC clients with OpenTracing and OpenTelemetry interceptors.
func TracingInterceptorDialOptions(opts ...otgrpc.Option) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(
			OpenTracingClientInterceptor(opts...),
			otelUnaryClientInterceptor(),
		),
		grpc.WithChainStreamInterceptor(
			OpenTracingStreamClientInterceptor(opts...),
			otelStreamClientInterceptor(),
		),
	}
}

// OpenTracingServerInterceptor provides a default gRPC server
// interceptor for emitting tracing to the global OpenTracing tracer.
func OpenTracingServerInterceptor(parentSpan opentracing.Span, options ...otgrpc.Option) grpc.UnaryServerInterceptor {
	options = append(options, logPayloads()...)
	tracer := opentracing.GlobalTracer()

	if parentSpan != nil {
		tracer = &reparentingTracer{parentSpan.Context(), tracer}
	}

	return otgrpc.OpenTracingServerInterceptor(tracer, options...)
}

// OpenTracingStreamServerInterceptor is OpenTracingServerInterceptor for instrumenting streaming gRPC calls.
func OpenTracingStreamServerInterceptor(parentSpan opentracing.Span,
	options ...otgrpc.Option,
) grpc.StreamServerInterceptor {
	options = append(options, logPayloads()...)
	tracer := opentracing.GlobalTracer()

	if parentSpan != nil {
		tracer = &reparentingTracer{parentSpan.Context(), tracer}
	}

	return otgrpc.OpenTracingStreamServerInterceptor(tracer, options...)
}

// OpenTracingClientInterceptor provides a default gRPC client interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingClientInterceptor(options ...otgrpc.Option) grpc.UnaryClientInterceptor {
	options = append(append(options,
		// Do not trace calls to the empty method
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ any) bool {
			return method != ""
		})), logPayloads()...)
	return otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer(), options...)
}

// OpenTracingStreamClientInterceptor is OpenTracingClientInterceptor for streaming gRPC calls.
func OpenTracingStreamClientInterceptor(options ...otgrpc.Option) grpc.StreamClientInterceptor {
	options = append(append(options,
		// Do not trace calls to the empty method
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ any) bool {
			return method != ""
		})), logPayloads()...)
	return otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer(), options...)
}

// Configures gRPC clients with OpenTracing interceptors.
func OpenTracingInterceptorDialOptions(opts ...otgrpc.Option) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(OpenTracingClientInterceptor(opts...)),
		grpc.WithChainStreamInterceptor(OpenTracingStreamClientInterceptor(opts...)),
	}
}

// Wraps an opentracing.Tracer to reparent orphan traces with a given
// default parent span.
type reparentingTracer struct {
	parentSpanContext opentracing.SpanContext
	underlying        opentracing.Tracer
}

func (t *reparentingTracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	if !t.hasChildOf(opts...) {
		opts = append(opts, opentracing.ChildOf(t.parentSpanContext))
	}
	return t.underlying.StartSpan(operationName, opts...)
}

func (t *reparentingTracer) Inject(sm opentracing.SpanContext, format any, carrier any) error {
	return t.underlying.Inject(sm, format, carrier)
}

func (t *reparentingTracer) Extract(format any, carrier any) (opentracing.SpanContext, error) {
	return t.underlying.Extract(format, carrier)
}

func (t *reparentingTracer) packOptions(opts ...opentracing.StartSpanOption) opentracing.StartSpanOptions {
	sso := opentracing.StartSpanOptions{}
	for _, o := range opts {
		o.Apply(&sso)
	}
	return sso
}

func (t *reparentingTracer) hasChildOf(opts ...opentracing.StartSpanOption) bool {
	for _, ref := range t.packOptions(opts...).References {
		if ref.Type == opentracing.ChildOfRef {
			return true
		}
	}
	return false
}

var _ opentracing.Tracer = &reparentingTracer{}

// Option to log payloads in trace spans. Default is on. Can be
// disabled by setting an env var to reduce tracing overhead.
func logPayloads() []otgrpc.Option {
	res := []otgrpc.Option{}
	if !cmdutil.IsTruthy(os.Getenv("PULUMI_TRACING_NO_PAYLOADS")) {
		res = append(res, otgrpc.LogPayloads())
	}
	return res
}

func captureStackTrace(skip int) string {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stackBuilder strings.Builder
	more := true
	for more {
		var frame runtime.Frame
		frame, more = frames.Next()
		fmt.Fprintf(&stackBuilder, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
	}

	return stackBuilder.String()
}

func otelUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = extractOTelContext(ctx)
		ctx, span := startServerSpan(ctx, info.FullMethod)
		defer span.End()

		resp, err := handler(ctx, req)
		setSpanStatus(span, err)
		return resp, err
	}
}

func otelStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := extractOTelContext(ss.Context())
		ctx, span := startServerSpan(ctx, info.FullMethod)
		defer span.End()

		wrapped := &serverStreamWithContext{ServerStream: ss, ctx: ctx}
		err := handler(srv, wrapped)
		setSpanStatus(span, err)
		return err
	}
}

// serverStreamWithContext wraps a grpc.ServerStream to provide a custom context.
type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context {
	return s.ctx
}

func extractOTelContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagationCarrier(md))
}

func startServerSpan(ctx context.Context, method string) (context.Context, trace.Span) {
	name := strings.TrimPrefix(method, "/")

	var attrs []attribute.KeyValue
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		attrs = []attribute.KeyValue{
			semconv.RPCSystemGRPC,
			semconv.RPCServiceKey.String(name[:idx]),
			semconv.RPCMethodKey.String(name[idx+1:]),
		}
	}

	tracer := otel.Tracer("pulumi-cli")
	return tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	)
}

func stackTraceUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() && span.IsRecording() {
			stackTrace := captureStackTrace(5)
			span.SetAttributes(attribute.String("code.stacktrace", stackTrace))
		}
		return handler(ctx, req)
	}
}

func stackTraceStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		span := trace.SpanFromContext(ss.Context())
		if span.SpanContext().IsValid() && span.IsRecording() {
			stackTrace := captureStackTrace(5)
			span.SetAttributes(attribute.String("code.stacktrace", stackTrace))
		}
		return handler(srv, ss)
	}
}

func otelUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx, span := startClientSpan(ctx, method, cc.Target())
		defer span.End()

		span.SetAttributes(attribute.String("code.stacktrace", captureStackTrace(4)))

		err := invoker(ctx, method, req, reply, cc, opts...)
		setSpanStatus(span, err)
		return err
	}
}

func otelStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx, span := startClientSpan(ctx, method, cc.Target())
		span.SetAttributes(attribute.String("code.stacktrace", captureStackTrace(4)))

		s, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			setSpanStatus(span, err)
			span.End()
			return s, err
		}
		return &trackedClientStream{ClientStream: s, span: span}, nil
	}
}

func startClientSpan(ctx context.Context, method, target string) (context.Context, trace.Span) {
	// Parse method name: "/package.Service/Method" -> "package.Service/Method"
	name := strings.TrimPrefix(method, "/")

	var attrs []attribute.KeyValue
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		attrs = []attribute.KeyValue{
			semconv.RPCSystemGRPC,
			semconv.RPCServiceKey.String(name[:idx]),
			semconv.RPCMethodKey.String(name[idx+1:]),
			attribute.String("net.peer.name", target),
		}
	}

	tracer := otel.Tracer("pulumi-cli")
	ctx, span := tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)

	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	otel.GetTextMapPropagator().Inject(ctx, propagationCarrier(md))
	return metadata.NewOutgoingContext(ctx, md), span
}

func setSpanStatus(span trace.Span, err error) {
	if err != nil {
		s, _ := status.FromError(err)
		span.SetStatus(codes.Error, s.Message())
		span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(s.Code())))
	} else {
		span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(grpccodes.OK)))
	}
}

// trackedClientStream wraps a grpc.ClientStream to end the span when the stream closes.
type trackedClientStream struct {
	grpc.ClientStream
	span trace.Span
}

func (s *trackedClientStream) RecvMsg(m any) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil && err != io.EOF {
		setSpanStatus(s.span, err)
		s.span.End()
	}
	return err
}

// propagationCarrier adapts gRPC metadata for trace context propagation.
type propagationCarrier metadata.MD

func (c propagationCarrier) Get(key string) string {
	if vals := metadata.MD(c).Get(key); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (c propagationCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

func (c propagationCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
