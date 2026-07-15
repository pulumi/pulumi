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

package plugin

import (
	"context"
	"log/slog"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"google.golang.org/protobuf/types/known/structpb"
)

var propertyLogMarshalOpts = MarshalOptions{
	KeepSecrets:      true,
	KeepUnknowns:     true,
	KeepOutputValues: true,
	// Suppress verbose marshal logging: this marshal runs *inside* the logging path (to build a log
	// attribute), and logging here would re-enter and recurse.
	skipLogging: true,
}

// PropertyExportHandler wraps the OTLP export handler, converting
// property-typed attributes into logging.PropertyValue for OTLP transport.
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
	sv := MarshalPropertyLogAttr(a)
	if sv == nil {
		return a
	}
	a.Value = slog.AnyValue(logging.PropertyValue{Key: a.Key, Value: sv})
	return a
}

// MarshalPropertyLogAttr marshals property-typed slog attributes into the
// protobuf value format used by Pulumi RPCs and OTLP log transport.
func MarshalPropertyLogAttr(a slog.Attr) *structpb.Value {
	if a.Value.Kind() != slog.KindAny {
		return nil
	}
	switch val := a.Value.Any().(type) {
	case *structpb.Struct:
		return structpb.NewStructValue(val)
	case resource.PropertyMap:
		return marshalResourceLogProperty(resource.NewProperty(val))
	case resource.PropertyValue:
		return marshalResourceLogProperty(val)
	case property.Map:
		return marshalResourceLogProperty(resource.NewProperty(resource.ToResourcePropertyMap(val)))
	case property.Value:
		return marshalResourceLogProperty(resource.ToResourcePropertyValue(val))
	default:
		return nil
	}
}

func marshalResourceLogProperty(pv resource.PropertyValue) *structpb.Value {
	sv, err := MarshalPropertyValue("", pv, propertyLogMarshalOpts)
	if err != nil || sv == nil {
		return nil
	}
	return sv
}
