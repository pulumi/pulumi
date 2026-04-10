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
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

// Receiver is an OTLP gRPC receiver that can receive traces, metrics, and logs
// from plugins and other processes.
type Receiver struct {
	server   *grpc.Server
	listener net.Listener
	port     int

	exporter SpanExporter

	// done signals when the server has stopped
	done chan struct{}
}

// traceService implements the OTLP TraceService.
type traceService struct {
	coltracepb.UnimplementedTraceServiceServer
	r *Receiver
}

// Start creates and starts a new OTLP receiver with the given exporter.
func Start(exporter SpanExporter) (*Receiver, error) {
	addr := "127.0.0.1:0"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	r := &Receiver{
		listener: listener,
		port:     port,
		exporter: exporter,
		done:     make(chan struct{}),
	}

	r.server = grpc.NewServer()

	coltracepb.RegisterTraceServiceServer(r.server, &traceService{r: r})

	go func() {
		defer close(r.done)
		if err := r.server.Serve(listener); err != nil && err != grpc.ErrServerStopped {
			slog.Info("OTLP receiver server error", "err", err)
		}
	}()

	slog.Info("OTLP receiver started", "port", port)

	return r, nil
}

func (r *Receiver) Endpoint() string {
	return fmt.Sprintf("localhost:%d", r.port)
}

func (r *Receiver) Shutdown(ctx context.Context) error {
	stopped := make(chan struct{})
	go func() {
		r.server.GracefulStop()
		close(stopped)
	}()

	var serverErr error
	select {
	case <-stopped:
		slog.Info("OTLP receiver stopped gracefully")
	case <-ctx.Done():
		r.server.Stop()
		slog.Info("OTLP receiver force stopped")
		serverErr = ctx.Err()
	}

	if r.exporter != nil {
		if err := r.exporter.Shutdown(ctx); err != nil {
			slog.Info("OTLP receiver: failed to shutdown exporter", "err", err)
			if serverErr == nil {
				serverErr = err
			}
		}
	}

	return serverErr
}

func (s *traceService) Export(
	ctx context.Context,
	req *coltracepb.ExportTraceServiceRequest,
) (*coltracepb.ExportTraceServiceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	slog.Debug("OTLP receiver: received resource spans", "count", len(req.ResourceSpans))

	if err := s.r.exporter.ExportSpans(ctx, req.ResourceSpans); err != nil {
		slog.Info("OTLP receiver: failed to export spans", "err", err)
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}
