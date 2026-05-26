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
	if a.Value.Kind() != slog.KindAny {
		return a
	}
	var pv resource.PropertyValue
	switch val := a.Value.Any().(type) {
	case resource.PropertyMap:
		pv = resource.NewProperty(val)
	case resource.PropertyValue:
		pv = val
	case property.Map:
		pv = resource.NewProperty(resource.ToResourcePropertyMap(val))
	case property.Value:
		pv = resource.ToResourcePropertyValue(val)
	default:
		return a
	}
	sv, err := plugin.MarshalPropertyValue("", pv, marshalOpts)
	if err != nil || sv == nil {
		return a
	}
	encoded, err := logging.EncodeStructValueForLog(sv)
	if err != nil {
		return a
	}
	a.Value = slog.AnyValue(encoded)
	return a
}
