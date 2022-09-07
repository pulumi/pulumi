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
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

// OpenTracingServerInterceptor provides a default gRPC server
// interceptor for emitting tracing to the global OpenTracing tracer.
func OpenTracingServerInterceptor(parentSpan opentracing.Span, options ...otgrpc.Option) grpc.UnaryServerInterceptor {
	// Log full payloads along with trace spans
	options = append(options, otgrpc.LogPayloads())
	tracer := opentracing.GlobalTracer()

	if parentSpan != nil {
		tracer = &reparentingTracer{parentSpan.Context(), tracer}
	}

	return otgrpc.OpenTracingServerInterceptor(tracer, options...)
}

// Like OpenTracingServerInterceptor but for instrumenting streaming gRPC calls.
func OpenTracingStreamServerInterceptor(parentSpan opentracing.Span, options ...otgrpc.Option) grpc.StreamServerInterceptor {
	// Log full payloads along with trace spans
	options = append(options, otgrpc.LogPayloads())
	tracer := opentracing.GlobalTracer()

	if parentSpan != nil {
		tracer = &reparentingTracer{parentSpan.Context(), tracer}
	}

	return otgrpc.OpenTracingStreamServerInterceptor(tracer, options...)
}

// OpenTracingClientInterceptor provides a default gRPC client interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingClientInterceptor(options ...otgrpc.Option) grpc.UnaryClientInterceptor {
	options = append(options,
		// Log full payloads along with trace spans
		otgrpc.LogPayloads(),
		// Do not trace calls to the empty method
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ interface{}) bool {
			return method != ""
		}))
	return otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer(), options...)
}

// Like OpenTracingClientInterceptor but for streaming gRPC calls.
func OpenTracingStreamClientInterceptor(options ...otgrpc.Option) grpc.StreamClientInterceptor {
	options = append(options,
		// Log full payloads along with trace spans
		otgrpc.LogPayloads(),
		// Do not trace calls to the empty method
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ interface{}) bool {
			return method != ""
		}))
	return otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer(), options...)
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
