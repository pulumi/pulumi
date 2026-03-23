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
	"os/user"
	"path/filepath"
	"strings"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func headersFromQuery(q url.Values) map[string]string {
	if len(q) == 0 {
		return nil
	}
	m := make(map[string]string, len(q))
	for k, vs := range q {
		if len(vs) > 0 {
			m[k] = vs[0]
		}
	}
	return m
}

type SpanExporter interface {
	// ExportSpans exports the given resource spans.
	ExportSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error
	// Shutdown shuts down the exporter.
	Shutdown(ctx context.Context) error
}

// NewExporter creates a SpanExporter based on the endpoint URL.
// Supported schemes:
//   - file:// - writes OTLP JSON to a local file
//   - grpc:// - sends OTLP via insecure gRPC (local collectors)
//   - grpcs:// - sends OTLP via TLS-secured gRPC with optional header auth
//
// grpc:// and grpcs:// support passing arbitrary gRPC metadata headers as
// URL query parameters:
//
//	grpcs://api.honeycomb.io:443?x-honeycomb-team=YOUR_API_KEY
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
		path, err := resolveFilePath(endpoint)
		if err != nil {
			return nil, err
		}
		return newFileExporter(path)

	case "grpc":
		host := u.Host
		if host == "" {
			return nil, errors.New("host is required for grpc:// endpoint")
		}
		return newGRPCExporterWithOptions(grpcExporterOptions{
			target:  host,
			headers: headersFromQuery(u.Query()),
		})

	case "grpcs":
		host := u.Host
		if host == "" {
			return nil, errors.New("host is required for grpcs:// endpoint")
		}
		return newGRPCExporterWithOptions(grpcExporterOptions{
			target:  host,
			tls:     true,
			headers: headersFromQuery(u.Query()),
		})

	default:
		return nil, fmt.Errorf("unsupported endpoint scheme: %s", u.Scheme)
	}
}

func resolveFilePath(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid file:// endpoint: %w", err)
	}

	// file://~/foo: tilde is in the Host field, path is /foo
	if u.Host == "~" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("could not determine current user to resolve file://~ path: %w", err)
		}
		path, err := filepath.Abs(filepath.Join(usr.HomeDir, filepath.FromSlash(strings.TrimPrefix(u.Path, "/"))))
		if err != nil {
			return "", fmt.Errorf("failed to resolve file path: %w", err)
		}
		return path, nil
	}

	// All other cases: authority is empty, full path is in u.Path. On Windows, file:///C:/foo parses to u.Path ==
	// "/C:/foo". Strip the leading slash before a drive letter so filepath.Abs doesn't prepend the current drive.
	p := u.Path
	if p == "" {
		return "", errors.New("file path is required for file:// endpoint")
	}
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		// looks like /X:/... — drop the leading slash
		p = p[1:]
	}

	path, err := filepath.Abs(filepath.FromSlash(p))
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}
	return path, nil
}
