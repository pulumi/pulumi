// Copyright 2016-2018, Pulumi Corporation.
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
	"strings"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// metadataReaderWriter satisfies both the opentracing.TextMapReader and
// opentracing.TextMapWriter interfaces.
type metadataReaderWriter struct {
	metadata.MD
}

func (w metadataReaderWriter) Set(key, val string) {
	// The GRPC HPACK implementation rejects any uppercase keys here.
	//
	// As such, since the HTTP_HEADERS format is case-insensitive anyway, we
	// blindly lowercase the key (which is guaranteed to work in the
	// Inject/Extract sense per the OpenTracing spec).
	key = strings.ToLower(key)
	w.MD[key] = append(w.MD[key], val)
}

func (w metadataReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range w.MD {
		for _, v := range vals {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}

	return nil
}

// OpenTracingServerInterceptor provides a default gRPC server interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingServerInterceptor(parentSpan opentracing.Span) grpc.UnaryServerInterceptor {
	tracingInterceptor := otgrpc.OpenTracingServerInterceptor(
		// Use the globally installed tracer
		opentracing.GlobalTracer(),
		// Log full payloads along with trace spans
		otgrpc.LogPayloads(),
	)
	if parentSpan == nil {
		return tracingInterceptor
	}
	spanContext := parentSpan.Context()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		carrier := metadataReaderWriter{md}
		_, err := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, carrier)
		if err == opentracing.ErrSpanContextNotFound {
			contract.IgnoreError(opentracing.GlobalTracer().Inject(spanContext, opentracing.HTTPHeaders, carrier))
		}
		return tracingInterceptor(ctx, req, info, handler)
	}

}

// OpenTracingClientInterceptor provides a default gRPC client interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingClientInterceptor() grpc.UnaryClientInterceptor {
	return otgrpc.OpenTracingClientInterceptor(
		// Use the globally installed tracer
		opentracing.GlobalTracer(),
		// Log full payloads along with trace spans
		otgrpc.LogPayloads(),
		// Do not trace calls to the empty method
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ interface{}) bool {
			return method != ""
		}))
}
