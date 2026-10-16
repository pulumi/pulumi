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

// Package logging implements the OTLP LogsService receiver that
// forwards log records to a LogExporter.
package logging

import (
	"context"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.opentelemetry.io/collector/pdata/plog"
)

// LogExporter receives OTLP log records.  Property value attributes
// are passed through as raw bytes; the consumer is responsible for
// decoding them (e.g. via logging.DecodeStructValueFromLog and
// plugin.UnmarshalProperties).
type LogExporter interface {
	ExportLogs(ctx context.Context, logs plog.Logs) error
	Shutdown(ctx context.Context) error
}

// Register registers the OTLP LogsService on a gRPC server,
// forwarding log records to the given exporter.
func Register(s *grpc.Server, exporter LogExporter) {
	collogspb.RegisterLogsServiceServer(s, &service{exporter: exporter})
}

type service struct {
	collogspb.UnimplementedLogsServiceServer
	exporter LogExporter
}

func (s *service) Export(
	ctx context.Context,
	req *collogspb.ExportLogsServiceRequest,
) (*collogspb.ExportLogsServiceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	protoBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal request: %v", err)
	}

	var unmarshaler plog.ProtoUnmarshaler
	logs, err := unmarshaler.UnmarshalLogs(protoBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal logs: %v", err)
	}

	if s.exporter != nil {
		if err := s.exporter.ExportLogs(ctx, logs); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to export logs: %v", err)
		}
	}

	return &collogspb.ExportLogsServiceResponse{}, nil
}
