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

// SlogLogExporter forwards OTLP log records into the slog default logger.
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
		attrs = append(attrs, key, logAttrValue(key, val))
		return true
	})

	slog.Log(context.Background(), level, msg, attrs...)
}

// logAttrValue converts an OTLP attribute back into the value the plugin
// originally logged, so that downstream handlers see the integer "v"
// verbosity attribute and structured values instead of their string forms.
func logAttrValue(key string, val pcommon.Value) any {
	switch val.Type() {
	case pcommon.ValueTypeBytes:
		if sv, err := logging.DecodeStructValueFromLog(val.Bytes().AsRaw()); err == nil {
			return logging.PropertyValue{Key: key, Value: sv}
		}
		return val.AsString()
	case pcommon.ValueTypeInt, pcommon.ValueTypeDouble, pcommon.ValueTypeBool,
		pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
		return val.AsRaw()
	case pcommon.ValueTypeEmpty, pcommon.ValueTypeStr:
		return val.AsString()
	default:
		return val.AsString()
	}
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
