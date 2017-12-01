// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package rpcutil

import (
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

// OpenTracingServerInterceptor provides a default gRPC server interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingServerInterceptor() grpc.UnaryServerInterceptor {
	return otgrpc.OpenTracingServerInterceptor(
		// Use the globally installed tracer
		opentracing.GlobalTracer(),
		// Log full payloads along with trace spans
		otgrpc.LogPayloads(),
	)
}

// OpenTracingClientInterceptor provides a default gRPC client interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingClientInterceptor() grpc.UnaryClientInterceptor {
	return otgrpc.OpenTracingClientInterceptor(
		// Use the globally installed tracer
		opentracing.GlobalTracer(),
		// Log full payloads along with trace spans
		otgrpc.LogPayloads(),
	)
}
