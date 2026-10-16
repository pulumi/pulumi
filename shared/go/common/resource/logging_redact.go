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

package resource

import (
	"log/slog"
)

// RedactedLogValue replaces secret values with "[secret]" for plaintext log output. The encrypted
// log sink and OTLP export receive the original values.
func (v PropertyValue) RedactedLogValue() slog.Value {
	return slog.AnyValue(redactSecretValues(v))
}

// RedactedLogValue replaces secret values with "[secret]" for plaintext log output. The encrypted
// log sink and OTLP export receive the original values.
func (m PropertyMap) RedactedLogValue() slog.Value {
	return slog.AnyValue(redactSecretValues(NewProperty(m)).ObjectValue())
}

func redactSecretValues(v PropertyValue) PropertyValue {
	switch {
	case v.IsSecret():
		return NewProperty("[secret]")
	case v.IsOutput():
		o := v.OutputValue()
		if o.Secret {
			return NewProperty("[secret]")
		}
		o.Element = redactSecretValues(o.Element)
		return NewProperty(o)
	case v.IsObject():
		obj := v.ObjectValue()
		redacted := make(PropertyMap, len(obj))
		for k, e := range obj {
			redacted[k] = redactSecretValues(e)
		}
		return NewProperty(redacted)
	case v.IsArray():
		arr := v.ArrayValue()
		redacted := make([]PropertyValue, len(arr))
		for i, e := range arr {
			redacted[i] = redactSecretValues(e)
		}
		return NewProperty(redacted)
	default:
		return v
	}
}
