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

package property

import (
	"log/slog"
)

// RedactedLogValue replaces secret values with "[secret]" for plaintext log output. The encrypted
// log sink and OTLP export receive the original values.
func (v Value) RedactedLogValue() slog.Value {
	return slog.AnyValue(redactSecretValues(v))
}

// RedactedLogValue replaces secret values with "[secret]" for plaintext log output. The encrypted
// log sink and OTLP export receive the original values.
func (m Map) RedactedLogValue() slog.Value {
	return slog.AnyValue(redactMapSecrets(m))
}

func redactSecretValues(v Value) Value {
	switch {
	case v.Secret():
		return New("[secret]")
	case v.IsMap():
		return New(redactMapSecrets(v.AsMap()))
	case v.IsArray():
		arr := v.AsArray().AsSlice()
		redacted := make([]Value, len(arr))
		for i, e := range arr {
			redacted[i] = redactSecretValues(e)
		}
		return New(NewArray(redacted))
	default:
		return v
	}
}

func redactMapSecrets(m Map) Map {
	orig := m.AsMap()
	redacted := make(map[string]Value, len(orig))
	for k, e := range orig {
		redacted[k] = redactSecretValues(e)
	}
	return NewMap(redacted)
}
