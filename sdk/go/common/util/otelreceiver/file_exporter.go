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
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// FileExporter writes OTLP trace data to a JSON file.
type FileExporter struct {
	mu   sync.Mutex
	file *os.File
}

// newFileExporter creates a new FileExporter that writes to the specified path.
func newFileExporter(path string) (*FileExporter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace file: %w", err)
	}
	return &FileExporter{file: f}, nil
}

// ExportSpans writes the given resource spans to the file in OTLP JSON format.
func (e *FileExporter) ExportSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error {
	if len(spans) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// This is slightly awkward, we marshal to proto first, and then
	// re-marshal to JSON later. Unfortunately this seems like the simplest
	// way to convert ResourceSpans to JSON.
	protoBytes, err := proto.Marshal(&coltracepb.ExportTraceServiceRequest{ResourceSpans: spans})
	if err != nil {
		return fmt.Errorf("failed to marshal spans: %w", err)
	}

	var unmarshaler ptrace.ProtoUnmarshaler
	traces, err := unmarshaler.UnmarshalTraces(protoBytes)
	if err != nil {
		return fmt.Errorf("failed to convert spans: %w", err)
	}

	var marshaler ptrace.JSONMarshaler
	jsonBytes, err := marshaler.MarshalTraces(traces)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if _, err := e.file.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}
	_, err = e.file.WriteString("\n")
	return err
}

func (e *FileExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	err := e.file.Close()
	e.file = nil
	return err
}
