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
	"net/url"
	"os"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	jaeger "github.com/uber/jaeger-client-go"
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
