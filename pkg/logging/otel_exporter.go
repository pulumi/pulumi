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

package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// SlogLogExporter is a LogExporter that forwards OTLP log records
// into the slog default logger.  Property value byte attributes are
// decoded and re-wrapped as logging.PropertyValue so they flow
// correctly through the handler chain: the sink handler encodes
// them to wire format, and the primary handler renders them as
// readable strings.
type SlogLogExporter struct{}

func (e *SlogLogExporter) ExportLogs(_ context.Context, logs plog.Logs) error {
	for i := range logs.ResourceLogs().Len() {
		rl := logs.ResourceLogs().At(i)
		for j := range rl.ScopeLogs().Len() {
			sl := rl.ScopeLogs().At(j)
			for k := range sl.LogRecords().Len() {
				e.exportRecord(sl.LogRecords().At(k))
			}
		}
	}
	return nil
}

func (e *SlogLogExporter) Shutdown(context.Context) error { return nil }

func (e *SlogLogExporter) exportRecord(lr plog.LogRecord) {
	level := otlpSeverityToSlog(lr.SeverityNumber())
	msg := lr.Body().AsString()

	attrs := make([]any, 0, lr.Attributes().Len()*2)
	lr.Attributes().Range(func(key string, val pcommon.Value) bool {
		if val.Type() == pcommon.ValueTypeBytes {
			raw := val.Bytes().AsRaw()
			sv, err := logging.DecodeStructValueFromLog(raw)
			if err == nil {
				attrs = append(attrs, key, logging.PropertyValue{Key: key, Value: sv})
				return true
			}
		}
		attrs = append(attrs, key, val.AsString())
		return true
	})

	slog.Log(context.Background(), level, msg, attrs...)
}

func otlpSeverityToSlog(sev plog.SeverityNumber) slog.Level {
	switch {
	case sev >= plog.SeverityNumberError:
		return slog.LevelError
	case sev >= plog.SeverityNumberWarn:
		return slog.LevelWarn
	case sev >= plog.SeverityNumberInfo:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}
