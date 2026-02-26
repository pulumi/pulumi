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

package otelreceiver

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type SpanExporter interface {
	// ExportSpans exports the given resource spans.
	ExportSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error
	// Shutdown shuts down the exporter.
	Shutdown(ctx context.Context) error
}

// NewExporter creates a SpanExporter based on the endpoint URL.
// Supported schemes:
//   - file:// - writes OTLP JSON to a local file
//   - grpc:// - sends OTLP via gRPC
//   - no scheme - defaults to gRPC
func NewExporter(endpoint string) (SpanExporter, error) {
	if endpoint == "" {
		return nil, errors.New("endpoint is required")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "file":
		path := u.Path
		if path == "" {
			path = u.Opaque
		}
		if path == "" {
			return nil, errors.New("file path is required for file:// endpoint")
		}
		return newFileExporter(path)

	case "grpc":
		host := u.Host
		if host == "" {
			return nil, errors.New("host is required for grpc:// endpoint")
		}
		return newGRPCExporter(host)

	case "":
		return newGRPCExporter(endpoint)

	default:
		return nil, fmt.Errorf("unsupported endpoint scheme: %s", u.Scheme)
	}
}
