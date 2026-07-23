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
	logProvider          *sdklog.LoggerProvider
	exportHandlerWrapper func(slog.Handler) slog.Handler
)

// RegisterExportHandlerWrapper registers a function that wraps the
// OTLP export handler when it is created.  Must be called before
// InitLogging.
func RegisterExportHandlerWrapper(wrap func(slog.Handler) slog.Handler) {
	exportHandlerWrapper = wrap
}

// initExportHandler sets up an OTLP log export handler if a log
// endpoint is available. Called from InitLogging.
func initExportHandler(serviceName string) {
	endpoint := os.Getenv("PULUMI_LOG_OTLP_ENDPOINT")
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
	var h slog.Handler = &propertyValueExportHandler{inner: inner}
	if exportHandlerWrapper != nil {
		h = exportHandlerWrapper(h)
	}
	SetExportHandler(h)
}

// shutdownExportHandler flushes and closes the OTLP log provider.
func shutdownExportHandler() {
	if logProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logProvider.Shutdown(ctx) //nolint:errcheck
		logProvider = nil
	}
	SetExportHandler(nil)
}

// PropertyValue wraps a *structpb.Value for use as an slog attribute value.
type PropertyValue struct {
	Key   string
	Value *structpb.Value
}

func (pv PropertyValue) String() string {
	b, err := json.Marshal(pv.Value.AsInterface())
	if err != nil {
		return "<error marshaling property value>"
	}
	return string(b)
}

func (pv PropertyValue) LogValue() slog.Value {
	return slog.StringValue(pv.String())
}

// RedactedLogValue replaces secret values with "[secret]" for plaintext log output. The encrypted
// log sink and OTLP export receive the original values.
func (pv PropertyValue) RedactedLogValue() slog.Value {
	b, err := json.Marshal(redactSecretsInJSON(pv.Value.AsInterface()))
	if err != nil {
		return slog.StringValue("<error marshaling property value>")
	}
	return slog.StringValue(string(b))
}

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
			encoded, err := EncodeStructValueForLog(pv.Value)
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
