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

package rpcutil

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

func TestOTelServerInterceptorFallbackParent(t *testing.T) {
	t.Parallel()

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prevProvider := otel.GetTracerProvider()
	prevPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevProvider)
		otel.SetTextMapPropagator(prevPropagator)
	})

	var fallback atomic.Pointer[trace.Span]
	provider := func() trace.Span {
		if s := fallback.Load(); s != nil {
			return *s
		}
		return nil
	}

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(TracingServerInterceptorOptionsWithOTelParent(nil, provider)...)
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := grpc_health_v1.NewHealthClient(conn)

	check := func(ctx context.Context) sdktrace.ReadOnlySpan {
		before := len(recorder.Ended())
		_, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		require.NoError(t, err)
		spans := recorder.Ended()
		require.Greater(t, len(spans), before)
		for _, span := range spans[before:] {
			if span.SpanKind() == trace.SpanKindServer {
				return span
			}
		}
		require.FailNow(t, "no server span recorded")
		return nil
	}

	tracer := tp.Tracer("test")

	_, spanA := tracer.Start(t.Context(), "parent-a")
	fallback.Store(&spanA)
	serverSpan := check(t.Context())
	require.Equal(t, spanA.SpanContext().SpanID(), serverSpan.Parent().SpanID())
	require.Equal(t, spanA.SpanContext().TraceID(), serverSpan.SpanContext().TraceID())

	_, spanB := tracer.Start(t.Context(), "parent-b")
	fallback.Store(&spanB)
	serverSpan = check(t.Context())
	require.Equal(t, spanB.SpanContext().SpanID(), serverSpan.Parent().SpanID())

	propagatedCtx, clientSpan := tracer.Start(t.Context(), "propagated-client")
	md := metadata.MD{}
	otel.GetTextMapPropagator().Inject(propagatedCtx, propagationCarrier(md))
	serverSpan = check(metadata.NewOutgoingContext(t.Context(), md))
	require.Equal(t, clientSpan.SpanContext().SpanID(), serverSpan.Parent().SpanID())
	require.Equal(t, clientSpan.SpanContext().TraceID(), serverSpan.SpanContext().TraceID())

	fallback.Store(nil)
	serverSpan = check(t.Context())
	require.False(t, serverSpan.Parent().IsValid())
}
