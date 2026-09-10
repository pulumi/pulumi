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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

type HTTPExporter struct {
	client  *http.Client
	url     string
	headers map[string]string
}

type httpExporterOptions struct {
	url     string
	headers map[string]string
}

func newHTTPExporterWithOptions(opts httpExporterOptions) *HTTPExporter {
	return &HTTPExporter{
		client:  &http.Client{Timeout: 30 * time.Second},
		url:     opts.url,
		headers: opts.headers,
	}
}

func (e *HTTPExporter) ExportSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error {
	if len(spans) == 0 {
		return nil
	}

	body, err := proto.Marshal(&coltracepb.ExportTraceServiceRequest{ResourceSpans: spans})
	if err != nil {
		return fmt.Errorf("failed to marshal spans: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to export spans: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("failed to export spans: %s: %s", resp.Status, msg)
	}
	return nil
}

func (e *HTTPExporter) Shutdown(ctx context.Context) error {
	e.client.CloseIdleConnections()
	return nil
}
