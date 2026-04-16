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

// Package logging implements the OTLP LogsService receiver that decodes
// property value attributes and forwards log records to a LogExporter.
package logging

import (
	"context"
	"encoding/json"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// LogExporter receives decoded OTLP log records. Property value
// attributes have already been converted from binary to JSON strings.
type LogExporter interface {
	ExportLogs(ctx context.Context, logs plog.Logs) error
	Shutdown(ctx context.Context) error
}

// NewRegistrar returns a ServiceRegistrar that registers the OTLP
// LogsService on a gRPC server, forwarding decoded log records to
// the given exporter.
func NewRegistrar(exporter LogExporter) func(*grpc.Server) {
	return func(s *grpc.Server) {
		collogspb.RegisterLogsServiceServer(s, &service{exporter: exporter})
	}
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

	decodePropertyValues(logs)

	if s.exporter != nil {
		if err := s.exporter.ExportLogs(ctx, logs); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to export logs: %v", err)
		}
	}

	return &collogspb.ExportLogsServiceResponse{}, nil
}

// decodePropertyValues walks all log record attributes and replaces
// any BytesValue that decodes as a property value (via the magic
// prefix in plugin.DecodePropertyValueFromLog) with its JSON string
// representation.
func decodePropertyValues(logs plog.Logs) {
	for i := range logs.ResourceLogs().Len() {
		rl := logs.ResourceLogs().At(i)
		for j := range rl.ScopeLogs().Len() {
			sl := rl.ScopeLogs().At(j)
			for k := range sl.LogRecords().Len() {
				decodeRecordAttrs(sl.LogRecords().At(k))
			}
		}
	}
}

func decodeRecordAttrs(lr plog.LogRecord) {
	lr.Attributes().Range(func(key string, val pcommon.Value) bool {
		if val.Type() == pcommon.ValueTypeBytes {
			raw := val.Bytes().AsRaw()
			pv, err := plugin.DecodePropertyValueFromLog(raw)
			if err == nil {
				b, err := json.Marshal(pv.Mappable())
				if err == nil {
					val.SetStr(string(b))
				}
			}
		}
		return true
	})
}
