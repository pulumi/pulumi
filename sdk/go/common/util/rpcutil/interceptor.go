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
	"os"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"google.golang.org/grpc"
)

// Configures interceptors to propagate OpenTracing metadata through headers. If parentSpan is non-nil, it becomes the
// default parent for orphan spans.
func OpenTracingServerInterceptorOptions(parentSpan opentracing.Span, options ...otgrpc.Option) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(OpenTracingServerInterceptor(parentSpan, options...)),
		grpc.ChainStreamInterceptor(OpenTracingStreamServerInterceptor(parentSpan, options...)),
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
	options ...otgrpc.Option) grpc.StreamServerInterceptor {

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
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ interface{}) bool {
			return method != ""
		})), logPayloads()...)
	return otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer(), options...)
}

// OpenTracingStreamClientInterceptor is OpenTracingClientInterceptor for streaming gRPC calls.
func OpenTracingStreamClientInterceptor(options ...otgrpc.Option) grpc.StreamClientInterceptor {
	options = append(append(options,
		// Do not trace calls to the empty method
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ interface{}) bool {
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

func (t *reparentingTracer) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return t.underlying.Inject(sm, format, carrier)
}

func (t *reparentingTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
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
