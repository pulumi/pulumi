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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	exportHandler slog.Handler
	logProvider   *sdklog.LoggerProvider
)

// initExportHandler checks for PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT and
// sets up an OTLP log export handler if present. Called from InitLogging.
func initExportHandler(serviceName string) {
	endpoint := os.Getenv("PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return
	}

	ctx := context.Background()

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return
	}

	exporter, err := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
	if err != nil {
		return
	}

	res, _ := resource.Merge(
		resource.Environment(),
		resource.NewWithAttributes("", semconv.ServiceName(serviceName)),
	)

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	logProvider = provider

	inner := otelslog.NewHandler(serviceName,
		otelslog.WithLoggerProvider(provider),
	)
	exportHandler = &propertyValueExportHandler{inner: inner}
}

// shutdownExportHandler flushes and closes the OTLP log provider.
func shutdownExportHandler() {
	if logProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logProvider.Shutdown(ctx) //nolint:errcheck
		logProvider = nil
	}
	exportHandler = nil
}

func logToExporter(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	h := exportHandler
	if h == nil || !h.Enabled(ctx, level) {
		return
	}
	r := slog.NewRecord(time.Now(), level, msg, 0)
	r.AddAttrs(attrs...)
	h.Handle(ctx, r) //nolint:errcheck
}

// PropertyValue wraps a *structpb.Struct for use as an slog attribute
// value.  When logged through the local handler it renders as JSON.
// When logged through the export handler it is encoded as a
// LogPropertyValue (protobuf bytes with a magic prefix) so the
// collector can identify and process it.
type PropertyValue struct {
	Key    string
	Struct *structpb.Struct
}

// String implements fmt.Stringer so that PropertyValue renders as JSON
// when used as a %v arg in Infof.
func (pv PropertyValue) String() string {
	b, err := json.Marshal(pv.Struct.AsMap())
	if err != nil {
		return "<error marshaling property value>"
	}
	return string(b)
}

// LogValue implements slog.LogValuer so the local slog handler gets a
// plain string.
func (pv PropertyValue) LogValue() slog.Value {
	return slog.StringValue(pv.String())
}

// NewPropertyValue creates a PropertyValue for use as an arg in Infof.
// The key is used as the attribute name when sent to the export handler.
// In the local log the value is rendered as JSON via fmt.Sprintf %v.
func NewPropertyValue(key string, s *structpb.Struct) PropertyValue {
	return PropertyValue{Key: key, Struct: s}
}

// replacePropertyValues re-formats the message, substituting each
// PropertyValue arg with a [[key]] placeholder and collecting the
// values as separate slog.Attr entries for the export handler.
func replacePropertyValues(format string, args []any) (string, []slog.Attr) {
	var attrs []slog.Attr
	has := false
	for _, arg := range args {
		if _, ok := arg.(PropertyValue); ok {
			has = true
			break
		}
	}
	if !has {
		return fmt.Sprintf(format, args...), nil
	}

	replaced := make([]any, len(args))
	for i, arg := range args {
		if pv, ok := arg.(PropertyValue); ok {
			replaced[i] = "[[" + pv.Key + "]]"
			attrs = append(attrs, slog.Any(pv.Key, pv))
		} else {
			replaced[i] = arg
		}
	}
	return fmt.Sprintf(format, replaced...), attrs
}

// propertyValueExportHandler wraps an slog.Handler and converts
// PropertyValue attrs to BytesValue before forwarding. All other
// attrs pass through unchanged.
type propertyValueExportHandler struct {
	inner slog.Handler
}

func (h *propertyValueExportHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *propertyValueExportHandler) Handle(ctx context.Context, r slog.Record) error {
	newRec := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if pv, ok := a.Value.Any().(PropertyValue); ok {
			sv := &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: pv.Struct}}
			encoded, err := EncodeStructValueForLog(sv)
			if err == nil {
				newRec.AddAttrs(slog.String(a.Key, string(encoded)))
			}
		} else {
			newRec.AddAttrs(a)
		}
		return true
	})
	return h.inner.Handle(ctx, newRec)
}

func (h *propertyValueExportHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &propertyValueExportHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *propertyValueExportHandler) WithGroup(name string) slog.Handler {
	return &propertyValueExportHandler{inner: h.inner.WithGroup(name)}
}
