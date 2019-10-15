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

package cmdutil

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/util/contract"
	jaeger "github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/thrift"
	"github.com/uber/jaeger-client-go/thrift-gen/zipkincore"
	"github.com/uber/jaeger-client-go/transport/zipkin"
	"sourcegraph.com/sourcegraph/appdash"
	appdash_opentracing "sourcegraph.com/sourcegraph/appdash/opentracing"
)

// TracingEndpoint is the Zipkin-compatible tracing endpoint where tracing data will be sent.
var TracingEndpoint string
var TracingToFile bool
var TracingRootSpan opentracing.Span

var traceCloser io.Closer

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

	// Store the tracing endpoint
	TracingEndpoint = tracingEndpoint

	endpointURL, err := url.Parse(tracingEndpoint)
	if err != nil {
		log.Fatalf("invalid tracing endpoint: %v", err)
	}

	var tracer opentracing.Tracer
	switch {
	case endpointURL.Scheme == "file":
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

		// If we are able to start a Zipkin endpoint that can be passed to other clients, record its endpoint as the
		// tracing endpoint.
		if addr, err := startZipkinAppdashServer(collector); err == nil {
			// Wrap the tracer in an implementation that will inject Jaeger as well as Appdash headers.
			tracer = newJaegerCodec(tracer)
			TracingEndpoint = addr
			TracingToFile = false
		}
	case endpointURL.Scheme == "tcp":
		// If the endpoint scheme is tcp, use an Appdash endpoint.
		collector := appdash.NewRemoteCollector(tracingEndpoint)
		traceCloser = collector
		tracer = appdash_opentracing.NewTracer(collector)
	default:
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
		TracingRootSpan = tracer.StartSpan(rootSpanName)
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

// jaegerCodec is an implementation of opentracing.Tracer that knows how to inject Jaeger span contexts alongside
// the span contexts of the wrapped tracer. This helps ensure that span contexts are carried across gRPC boundaries
// if the local tracer is the Appdash tracer, but we are accepting spans from remote Jaeger clients.
type jaegerCodec struct {
	tracer       opentracing.Tracer
	jaegerTracer opentracing.Tracer
}

func newJaegerCodec(tracer opentracing.Tracer) opentracing.Tracer {
	jaegerTracer, _ := jaeger.NewTracer("", jaeger.NewConstSampler(false), jaeger.NewNullReporter())
	return &jaegerCodec{
		tracer:       tracer,
		jaegerTracer: jaegerTracer,
	}
}

func (j *jaegerCodec) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	return j.tracer.StartSpan(operationName, opts...)
}

func (j *jaegerCodec) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	if err := j.tracer.Inject(sm, format, carrier); err != nil {
		return err
	}

	var jaegerSpanContext jaeger.SpanContext
	switch sm := sm.(type) {
	case jaeger.SpanContext:
		jaegerSpanContext = sm
	case basictracer.SpanContext:
		traceID := jaeger.TraceID{Low: sm.TraceID}
		jaegerSpanContext = jaeger.NewSpanContext(traceID, jaeger.SpanID(sm.SpanID), 0, sm.Sampled, sm.Baggage)
	default:
		// Cannot inject this sort of span.
		return nil
	}

	return j.jaegerTracer.Inject(jaegerSpanContext, format, carrier)
}

func (j *jaegerCodec) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	if spanCtx, err := j.tracer.Extract(format, carrier); err == nil {
		// Successfully extracted a trace. Carry on.
		return spanCtx, nil
	}

	// Otherwise, attempt to extract a Jaeger trace.
	return j.jaegerTracer.Extract(format, carrier)
}

// httpRequestTransport is a Thrift transport backed by an HTTP request.
type httpRequestTransport struct {
	req  *http.Request
	read int64
}

func (t *httpRequestTransport) Read(p []byte) (n int, err error) {
	n, err = t.req.Body.Read(p)
	t.read += int64(n)
	return n, err
}

func (t *httpRequestTransport) Write(p []byte) (n int, err error) {
	panic("not implemented")
}

func (t *httpRequestTransport) Close() error {
	return t.req.Body.Close()
}

func (t *httpRequestTransport) Flush() (err error) {
	panic("not implemented")
}

func (t *httpRequestTransport) RemainingBytes() (num_bytes uint64) {
	return uint64(t.req.ContentLength - t.read)
}

func (t *httpRequestTransport) Open() error {
	return nil
}

func (t *httpRequestTransport) IsOpen() bool {
	return true
}

// parseRequestBody parses the body of the given Zipkin tracing request into a list of Spans.
func parseRequestBody(req *http.Request) ([]zipkincore.Span, error) {
	p := thrift.NewTBinaryProtocolTransport(&httpRequestTransport{req: req})

	_, n, err := p.ReadListBegin()
	if err != nil {
		return nil, err
	}

	spans := make([]zipkincore.Span, n)
	for i := 0; i < len(spans); i++ {
		if err := spans[i].Read(p); err != nil {
			return nil, err
		}
	}

	if err := p.ReadListEnd(); err != nil {
		return nil, err
	}

	return spans, nil
}

// startZipkinAppdashServer starts a service that listens for traces submitted by remote clients using the Zipkin v2
// protocol over HTTP + Thrift. It returns the address of the server.
func startZipkinAppdashServer(collector appdash.Collector) (string, error) {
	l, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return "", err
	}

	recorder := appdash_opentracing.NewRecorder(collector, appdash_opentracing.Options{})
	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		spans, err := parseRequestBody(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, zs := range spans {
			parentSpanID := uint64(0)
			if zs.ParentID != nil {
				parentSpanID = uint64(*zs.ParentID)
			}

			var start time.Time
			if zs.Timestamp != nil {
				start = time.Unix(0, *zs.Timestamp*int64(time.Microsecond/time.Nanosecond))
			}
			var duration time.Duration
			if zs.Duration != nil {
				duration = time.Duration(*zs.Duration) * (time.Microsecond / time.Nanosecond)
			}

			// TODO: parse annotations
			span := basictracer.RawSpan{
				Context: basictracer.SpanContext{
					TraceID: uint64(zs.TraceID),
					SpanID:  uint64(zs.ID),
					Sampled: true,
				},
				ParentSpanID: parentSpanID,
				Operation:    zs.Name,
				Start:        start,
				Duration:     duration,
			}
			recorder.RecordSpan(span)
			log.Printf("recorded span %#v -> %#v", zs, span)
		}

		w.WriteHeader(http.StatusOK)
	}))

	return "http://" + l.Addr().String(), nil
}
