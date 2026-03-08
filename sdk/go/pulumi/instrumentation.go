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

package pulumi

import (
	"context"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var tracerProvider *sdktrace.TracerProvider

// initTracing initializes OpenTelemetry tracing when TRACEPARENT and
// OTEL_EXPORTER_OTLP_ENDPOINT environment variables are present.
// Returns a context with the extracted trace parent.
func initTracing(ctx context.Context) context.Context {
	traceparent := os.Getenv("TRACEPARENT")
	if traceparent == "" {
		return ctx
	}

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		return ctx
	}

	conn, err := grpc.NewClient(otlpEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logging.V(3).Infof("pulumi-sdk-go: failed to create gRPC connection for tracing: %v", err)
		return ctx
	}

	exporter, err := otlptracegrpc.New(context.Background(), otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		logging.V(3).Infof("pulumi-sdk-go: failed to create OTLP exporter: %v", err)
		return ctx
	}

	res := resource.NewWithAttributes("", semconv.ServiceName("pulumi-sdk-go"))
	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	carrier := propagation.MapCarrier{"traceparent": traceparent}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func shutdownTracing() {
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(context.Background()); err != nil {
			logging.V(3).Infof("pulumi-sdk-go: failed to shutdown tracer provider: %v", err)
		}
	}
}
