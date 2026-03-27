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

package provider

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"go.opentelemetry.io/otel"
	otbridge "go.opentelemetry.io/otel/bridge/opentracing"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Tracing is the optional command line flag passed to this provider for configuring a  Zipkin-compatible tracing
// endpoint
var tracing string

// Main is the typical entrypoint for a resource provider plugin.  Using it isn't required but can cut down
// significantly on the amount of boilerplate necessary to fire up a new resource provider.
func Main(name string, provMaker func(*HostClient) (pulumirpc.ResourceProviderServer, error)) error {
	return MainContext(context.Background(), name, provMaker)
}

// MainContext is the same as Main but it accepts a context so it can be cancelled.
func MainContext(
	ctx context.Context,
	name string,
	provMaker func(*HostClient) (pulumirpc.ResourceProviderServer, error),
) error {
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.Parse()

	// Initialize loggers before going any further.
	logging.InitLogging(false, 0, false)

	// When the CLI provides an OTel endpoint, we use OTel as the primary tracing backend and bridge legacy OpenTracing
	// calls through it so all spans end up in the same OTel trace. Otherwise fall back to legacy OpenTracing/AppDash.
	otelEP := os.Getenv("PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT")
	var serverOpts []grpc.ServerOption
	if otelEP != "" {
		if err := cmdutil.InitOtelTracing(name, otelEP); err != nil {
			logging.V(3).Infof("failed to initialize OTel tracing: %v", err)
		} else {
			defer cmdutil.CloseOtelTracing()

			// The otbridge tracer forwards OpenTracing API calls to the OpenTelemetry SDK. This allows providers that
			// are instrumented using OpenTracing to be seamlessly integrated with our OTel traces. Eventually we might
			// want to replace this code in the bridge and/or providers to actually use OTel, which well let us drop
			// this bridge.
			bridgeTracer := otbridge.NewBridgeTracer()
			bridgeTracer.SetOpenTelemetryTracer(otel.Tracer(name))
			opentracing.SetGlobalTracer(bridgeTracer)

			serverOpts = rpcutil.OTelServerInterceptorOptions()
			// We need to add the OTel span to the context so that the bridgeTracer can properly attach the spans to it.
			bridgeUnary := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
				if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
					ctx = bridgeTracer.ContextWithBridgeSpan(ctx, span)
				}
				return handler(ctx, req)
			}
			bridgeStream := func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				ctx := ss.Context()
				if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
					ctx = bridgeTracer.ContextWithBridgeSpan(ctx, span)
					ss = &wrappedServerStream{ServerStream: ss, ctx: ctx}
				}
				return handler(srv, ss)
			}
			serverOpts = append(serverOpts,
				grpc.ChainUnaryInterceptor(bridgeUnary),
				grpc.ChainStreamInterceptor(bridgeStream),
			)
		}
	} else {
		cmdutil.InitTracing(name, name, tracing)
		serverOpts = rpcutil.OpenTracingServerInterceptorOptions(nil)
	}

	// When the engine is done with this provider it sends SIGINT. We catch the signalhere to trigger a graceful
	// shutdown: we cancel the ctx, the cancelChannel closes, the gRPC server calls GracefulStop, handle.Done at the
	// bottom of this function unblocks and returns, and finally the deferred CloseOtelTracing flushes any buffered
	// spans before the process exits.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	// Read the non-flags args and connect to the engine.
	args := flag.Args()
	var host *HostClient
	if len(args) == 0 {
		// Start the provider in Attach mode
	} else if len(args) == 1 {
		var err error
		host, err = NewHostClient(args[0])
		if err != nil {
			return fmt.Errorf("fatal: could not connect to host RPC: %w", err)
		}

		// If we have a host cancel our cancellation context if it fails the healthcheck
		ctx, cancel = context.WithCancel(ctx)
		// map the context Done channel to the rpcutil boolean cancel channel
		err = rpcutil.Healthcheck(ctx, args[0], 5*time.Minute, cancel)
		if err != nil {
			return fmt.Errorf("could not start health check host RPC server: %w", err)
		}
	} else {
		return errors.New("fatal: could not connect to host RPC; missing argument")
	}

	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		close(cancelChannel)
	}()

	// Fire up a gRPC server, letting the kernel choose a free port for us.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			prov, proverr := provMaker(host)
			if proverr != nil {
				return fmt.Errorf("failed to create resource provider: %v", proverr)
			}
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
		Options: serverOpts,
	})
	if err != nil {
		return fmt.Errorf("fatal: %w", err)
	}

	// The resource provider protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%d\n", handle.Port)

	// Finally, wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		return fmt.Errorf("fatal: %w", err)
	}

	return nil
}

// wrappedServerStream overrides the context of a grpc.ServerStream.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *wrappedServerStream) Context() context.Context { return s.ctx }
