// Copyright 2016-2021, Pulumi Corporation.
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

package cmdutil

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/appdash"
	appdash_opentracing "github.com/pulumi/appdash/opentracing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/otelreceiver"
	jaeger "github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport/zipkin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TracingEndpoint is the Zipkin-compatible tracing endpoint where tracing data will be sent.
var TracingEndpoint string

// TracingToFile indicates if pulumi was called with a file:// scheme URL (--tracing=file:///...).
//
// Deprecated: Even in this case TracingEndpoint will now have the tcp:// scheme and will point to a
// proxy server that will append traces to the user-specified file. Plugins should respect
// TracingEndpoint and ignore TracingToFile.
var TracingToFile bool

var TracingRootSpan opentracing.Span

var traceCloser io.Closer

// otelEndpoint is the OTLP gRPC endpoint where plugins should send OpenTelemetry telemetry.
var otelEndpoint string

var (
	otelReceiver       *otelreceiver.Receiver
	otelTracerProvider *sdktrace.TracerProvider
)

type localStore struct {
	path  string
	store *appdash.MemoryStore
}

func (s *localStore) Close() error {
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(f)
	return s.store.Write(f)
}

func IsTracingEnabled() bool {
	return TracingEndpoint != ""
}

// InitTracing initializes tracing
func InitTracing(name, rootSpanName, tracingEndpoint string) {
	// If no tracing endpoint was provided, just return. The default global tracer is already a no-op tracer.
	if tracingEndpoint == "" {
		return
	}

	endpointURL, err := url.Parse(tracingEndpoint)
	if err != nil {
		log.Fatalf("invalid tracing endpoint: %v", err)
	}

	var tracer opentracing.Tracer
	switch endpointURL.Scheme {
	case "file":
		// If the endpoint is a file:// URL, use a local tracer.
		TracingToFile = true

		path := endpointURL.Path
		if path == "" {
			path = endpointURL.Opaque
		}
		if path == "" {
			log.Fatalf("invalid tracing endpoint: %v", err)
		}

		store := &localStore{
			path:  path,
			store: appdash.NewMemoryStore(),
		}
		traceCloser = store

		collector := appdash.NewLocalCollector(store.store)
		tracer = appdash_opentracing.NewTracer(collector)

		proxyEndpoint, err := startProxyAppDashServer(collector)
		if err != nil {
			log.Fatal(err)
		}

		// Instead of storing the original endpoint, store the
		// proxy endpoint. The TracingEndpoint global var is
		// consumed by code forking off sub-processes, and we
		// want those sending data to the proxy endpoint, so
		// it cleanly lands in the file managed by the parent
		// process.
		TracingEndpoint = proxyEndpoint

	case "tcp":
		// Store the tracing endpoint
		TracingEndpoint = tracingEndpoint

		// If the endpoint scheme is tcp, use an Appdash endpoint.
		collector := appdash.NewRemoteCollector(endpointURL.Host)
		traceCloser = collector
		tracer = appdash_opentracing.NewTracer(collector)

	default:
		// Store the tracing endpoint
		TracingEndpoint = tracingEndpoint

		// Jaeger tracer can be initialized with a transport that will
		// report tracing Spans to a Zipkin backend
		transport, err := zipkin.NewHTTPTransport(
			tracingEndpoint,
			zipkin.HTTPBatchSize(1),
			zipkin.HTTPLogger(jaeger.StdLogger),
		)
		if err != nil {
			log.Fatalf("Cannot initialize HTTP transport: %v", err)
		}

		// create Jaeger tracer
		t, closer := jaeger.NewTracer(
			name,
			jaeger.NewConstSampler(true), // sample all traces
			jaeger.NewRemoteReporter(transport))

		tracer, traceCloser = t, closer
	}

	// Set the ambient tracer
	opentracing.SetGlobalTracer(tracer)

	// If a root span was requested, start it now.
	if rootSpanName != "" {
		var options []opentracing.StartSpanOption
		for _, tag := range rootSpanTags() {
			options = append(options, tag)
		}
		TracingRootSpan = tracer.StartSpan(rootSpanName, options...)
		go collectMemStats(rootSpanName)
	}
}

// CloseTracing ensures that all pending spans have been flushed.  It should be called before process exit.
func CloseTracing() {
	if !IsTracingEnabled() {
		return
	}

	if TracingRootSpan != nil {
		TracingRootSpan.Finish()
	}

	contract.IgnoreClose(traceCloser)
}

// IsOTelEnabled returns true if OTEL is enabled via environment variable or endpoint is set.
func IsOTelEnabled() bool {
	return otelEndpoint != ""
}

// InitOTelReceiver starts the OTLP receiver with the given endpoint.
func InitOtelReceiver(endpoint string) error {
	if endpoint == "" {
		return nil
	}

	exporter, err := otelreceiver.NewExporter(endpoint)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	otelReceiver, err = otelreceiver.Start(exporter)
	if err != nil {
		_ = exporter.Shutdown(context.Background())
		return fmt.Errorf("failed to start OTLP receiver: %w", err)
	}

	otelEndpoint = otelReceiver.Endpoint()
	logging.V(5).Infof("Started local OTLP receiver at %s with exporter for %s", otelEndpoint, endpoint)

	// Set up Otel TracerProvider for CLI's own spans
	// The CLI sends its spans to the local receiver, which forwards to the configured exporter
	if err := initOtelTracerProvider(otelEndpoint); err != nil {
		logging.V(3).Infof("failed to initialize OTel tracer provider: %v", err)
	}

	return nil
}

// initOTelTracerProviderForService sets up the global OTel TracerProvider.
func initOtelTracerProvider(endpoint string) error {
	ctx := context.Background()

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res := resource.NewWithAttributes(
		"",
		semconv.ServiceName("pulumi-cli"),
	)

	otelTracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(otelTracerProvider)

	return nil
}

func CloseOTelReceiver() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if otelTracerProvider != nil {
		if err := otelTracerProvider.Shutdown(ctx); err != nil {
			logging.V(3).Infof("error closing OTel tracer provider: %v", err)
		}
		otelTracerProvider = nil
	}

	if otelReceiver != nil {
		if err := otelReceiver.Shutdown(ctx); err != nil {
			logging.V(3).Infof("error closing OTLP receiver: %v", err)
		}
		otelReceiver = nil
	}

	otelEndpoint = ""
}

// Starts an AppDash server listening on any available TCP port
// locally and sends the spans and annotations to the given collector.
// Returns a Pulumi-formatted tracing endpoint pointing to this
// server.
//
// See https://github.com/sourcegraph/appdash/blob/master/cmd/appdash/example_app.go
func startProxyAppDashServer(collector appdash.Collector) (string, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		return "", err
	}
	collectorPort := l.Addr().(*net.TCPAddr).Port

	cs := appdash.NewServer(l, collector)
	cs.Debug = true
	cs.Trace = true
	go cs.Start()

	// The default sends to stderr, which is unfortunate for
	// end-users. Discard for now.
	cs.Log = log.New(io.Discard, "appdash", 0)

	return fmt.Sprintf("tcp://127.0.0.1:%d", collectorPort), nil
}

// Computes initial tags to write to the `TracingRootSpan`, which can
// be useful for aggregating trace data in benchmarks.
func rootSpanTags() []opentracing.Tag {
	tags := []opentracing.Tag{
		{
			Key:   "os.Args",
			Value: os.Args,
		},
		{
			Key:   "runtime.GOOS",
			Value: runtime.GOOS,
		},
		{
			Key:   "runtime.GOARCH",
			Value: runtime.GOARCH,
		},
		{
			Key:   "runtime.GOMAXPROCS",
			Value: runtime.GOMAXPROCS(0),
		},
		{
			Key:   "runtime.NumCPU",
			Value: runtime.NumCPU(),
		},
	}

	// Promote all env vars `pulumi_tracing_tag_foo=bar` into tags `foo: bar`.
	envPrefix := "pulumi_tracing_tag_"
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		envVarName := strings.ToLower(pair[0])
		envVarValue := pair[1]

		if strings.HasPrefix(envVarName, envPrefix) {
			tags = append(tags, opentracing.Tag{
				Key:   strings.TrimPrefix(envVarName, envPrefix),
				Value: envVarValue,
			})
		}
	}

	return tags
}

// Samples memory stats in the background at 1s intervals, and creates
// spans for the data. This is currently opt-in via
// `PULUMI_TRACING_MEMSTATS_POLL_INTERVAL=1s` or similar. Consider
// collecting this by default later whenever tracing is enabled as we
// calibrate that the overhead is low enough.
func collectMemStats(spanPrefix string) {
	memStats := runtime.MemStats{}
	maxStats := runtime.MemStats{}

	poll := func() {
		if TracingRootSpan == nil {
			return
		}

		runtime.ReadMemStats(&memStats)

		// report cumulative metrics as is
		TracingRootSpan.SetTag("runtime.NumCgoCall", runtime.NumCgoCall())
		TracingRootSpan.SetTag("MemStats.TotalAlloc", memStats.TotalAlloc)
		TracingRootSpan.SetTag("MemStats.Mallocs", memStats.Mallocs)
		TracingRootSpan.SetTag("MemStats.Frees", memStats.Frees)
		TracingRootSpan.SetTag("MemStats.PauseTotalNs", memStats.PauseTotalNs)
		TracingRootSpan.SetTag("MemStats.NumGC", memStats.NumGC)

		// for other metrics report the max

		if memStats.Sys > maxStats.Sys {
			maxStats.Sys = memStats.Sys
			TracingRootSpan.SetTag("MemStats.Sys.Max", maxStats.Sys)
		}

		if memStats.HeapAlloc > maxStats.HeapAlloc {
			maxStats.HeapAlloc = memStats.HeapAlloc
			TracingRootSpan.SetTag("MemStats.HeapAlloc.Max", maxStats.HeapAlloc)
		}

		if memStats.HeapSys > maxStats.HeapSys {
			maxStats.HeapSys = memStats.HeapSys
			TracingRootSpan.SetTag("MemStats.HeapSys.Max", maxStats.HeapSys)
		}

		if memStats.HeapIdle > maxStats.HeapIdle {
			maxStats.HeapIdle = memStats.HeapIdle
			TracingRootSpan.SetTag("MemStats.HeapIdle.Max", maxStats.HeapIdle)
		}

		if memStats.HeapInuse > maxStats.HeapInuse {
			maxStats.HeapInuse = memStats.HeapInuse
			TracingRootSpan.SetTag("MemStats.HeapInuse.Max", maxStats.HeapInuse)
		}

		if memStats.HeapReleased > maxStats.HeapReleased {
			maxStats.HeapReleased = memStats.HeapReleased
			TracingRootSpan.SetTag("MemStats.HeapReleased.Max", maxStats.HeapReleased)
		}

		if memStats.HeapObjects > maxStats.HeapObjects {
			maxStats.HeapObjects = memStats.HeapObjects
			TracingRootSpan.SetTag("MemStats.HeapObjects.Max", maxStats.HeapObjects)
		}

		if memStats.StackInuse > maxStats.StackInuse {
			maxStats.StackInuse = memStats.StackInuse
			TracingRootSpan.SetTag("MemStats.StackInuse.Max", maxStats.StackInuse)
		}

		if memStats.StackSys > maxStats.StackSys {
			maxStats.StackSys = memStats.StackSys
			TracingRootSpan.SetTag("MemStats.StackSys.Max", maxStats.StackSys)
		}
	}

	interval := os.Getenv("PULUMI_TRACING_MEMSTATS_POLL_INTERVAL")

	if interval != "" {
		intervalDuration, err := time.ParseDuration(interval)
		if err == nil {
			for {
				poll()
				time.Sleep(intervalDuration)
			}
		}
	}
}
