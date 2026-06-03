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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"google.golang.org/protobuf/types/known/structpb"
)

var marshalOpts = plugin.MarshalOptions{
	KeepSecrets:      true,
	KeepUnknowns:     true,
	KeepOutputValues: true,
}

// PropertySinkHandler wraps the encrypted log sink handler.  It
// encodes resource.PropertyMap, resource.PropertyValue, property.Map,
// and property.Value attributes into the [magic][protobuf] wire
// format so they can be decoded later by the decrypt command.
// Already-encoded bytes (from OTLP) are passed through as-is.
type PropertySinkHandler struct {
	inner slog.Handler
}

func NewPropertySinkHandler(inner slog.Handler) *PropertySinkHandler {
	return &PropertySinkHandler{inner: inner}
}

func (h *PropertySinkHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *PropertySinkHandler) Handle(ctx context.Context, r slog.Record) error {
	newRec := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRec.AddAttrs(h.encodeAttr(a))
		return true
	})
	return h.inner.Handle(ctx, newRec)
}

func (h *PropertySinkHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	encoded := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		encoded[i] = h.encodeAttr(a)
	}
	return &PropertySinkHandler{inner: h.inner.WithAttrs(encoded)}
}

func (h *PropertySinkHandler) WithGroup(name string) slog.Handler {
	return &PropertySinkHandler{inner: h.inner.WithGroup(name)}
}

func (h *PropertySinkHandler) encodeAttr(a slog.Attr) slog.Attr {
	sv := marshalPropertyAttr(a)
	if sv == nil {
		return a
	}
	encoded, err := logging.EncodeStructValueForLog(sv)
	if err != nil {
		return a
	}
	a.Value = slog.AnyValue(encoded)
	return a
}

// PropertyExportHandler wraps the OTLP export handler.  It converts
// resource.PropertyMap, resource.PropertyValue, property.Map, and
// property.Value attributes into logging.PropertyValue so the
// downstream export handler can encode them for OTLP transport.
type PropertyExportHandler struct {
	inner slog.Handler
}

func NewPropertyExportHandler(inner slog.Handler) *PropertyExportHandler {
	return &PropertyExportHandler{inner: inner}
}

func (h *PropertyExportHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *PropertyExportHandler) Handle(ctx context.Context, r slog.Record) error {
	newRec := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRec.AddAttrs(h.wrapAttr(a))
		return true
	})
	return h.inner.Handle(ctx, newRec)
}

func (h *PropertyExportHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	wrapped := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		wrapped[i] = h.wrapAttr(a)
	}
	return &PropertyExportHandler{inner: h.inner.WithAttrs(wrapped)}
}

func (h *PropertyExportHandler) WithGroup(name string) slog.Handler {
	return &PropertyExportHandler{inner: h.inner.WithGroup(name)}
}

func (h *PropertyExportHandler) wrapAttr(a slog.Attr) slog.Attr {
	sv := marshalPropertyAttr(a)
	if sv == nil {
		return a
	}
	a.Value = slog.AnyValue(logging.PropertyValue{Key: a.Key, Value: sv})
	return a
}

func marshalPropertyAttr(a slog.Attr) *structpb.Value {
	if a.Value.Kind() != slog.KindAny {
		return nil
	}
	switch val := a.Value.Any().(type) {
	case *structpb.Struct:
		return structpb.NewStructValue(val)
	case resource.PropertyMap:
		return marshalResourceProperty(resource.NewProperty(val))
	case resource.PropertyValue:
		return marshalResourceProperty(val)
	case property.Map:
		return marshalResourceProperty(resource.NewProperty(resource.ToResourcePropertyMap(val)))
	case property.Value:
		return marshalResourceProperty(resource.ToResourcePropertyValue(val))
	default:
		return nil
	}
}

func marshalResourceProperty(pv resource.PropertyValue) *structpb.Value {
	sv, err := plugin.MarshalPropertyValue("", pv, marshalOpts)
	if err != nil || sv == nil {
		return nil
	}
	return sv
}
