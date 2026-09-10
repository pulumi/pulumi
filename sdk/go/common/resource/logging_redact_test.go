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
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// logRedactAttr mirrors what the logging package's primary output does with an attribute:
// values implementing RedactedLogValue are replaced by their redacted form.
func logRedactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() != slog.KindAny {
		return a
	}
	if v, ok := a.Value.Any().(interface{ RedactedLogValue() slog.Value }); ok {
		a.Value = v.RedactedLogValue()
	}
	return a
}

func TestRedactSecretAttr(t *testing.T) {
	t.Parallel()

	t.Run("secret property value", func(t *testing.T) {
		t.Parallel()
		a := logRedactAttr(slog.Any("value", MakeSecret(NewProperty("hunter2"))))
		pv, ok := a.Value.Any().(PropertyValue)
		require.True(t, ok)
		assert.Equal(t, NewProperty("[secret]"), pv)
	})

	t.Run("nested secrets in a property map", func(t *testing.T) {
		t.Parallel()
		m := PropertyMap{
			"plain": NewProperty("visible"),
			"token": MakeSecret(NewProperty("hunter2")),
			"nested": NewProperty(PropertyMap{
				"password": MakeSecret(NewProperty("hunter2")),
			}),
			"list": NewProperty([]PropertyValue{MakeSecret(NewProperty("hunter2"))}),
		}
		a := logRedactAttr(slog.Any("props", m))
		redacted, ok := a.Value.Any().(PropertyMap)
		require.True(t, ok)
		assert.Equal(t, NewProperty("visible"), redacted["plain"])
		assert.Equal(t, NewProperty("[secret]"), redacted["token"])
		assert.Equal(t, NewProperty("[secret]"), redacted["nested"].ObjectValue()["password"])
		assert.Equal(t, NewProperty("[secret]"), redacted["list"].ArrayValue()[0])
		assert.Equal(t, MakeSecret(NewProperty("hunter2")), m["token"])
	})

	t.Run("secret output value", func(t *testing.T) {
		t.Parallel()
		a := logRedactAttr(slog.Any("value", NewProperty(Output{
			Element: NewProperty("hunter2"),
			Known:   true,
			Secret:  true,
		})))
		pv, ok := a.Value.Any().(PropertyValue)
		require.True(t, ok)
		assert.Equal(t, NewProperty("[secret]"), pv)
	})

	t.Run("secret property.Value", func(t *testing.T) {
		t.Parallel()
		a := logRedactAttr(slog.Any("value", property.New("hunter2").WithSecret(true)))
		pv, ok := a.Value.Any().(property.Value)
		require.True(t, ok)
		assert.Equal(t, property.New("[secret]"), pv)
	})

	t.Run("non-property attributes pass through", func(t *testing.T) {
		t.Parallel()
		a := logRedactAttr(slog.String("plain", "hunter2"))
		assert.Equal(t, "hunter2", a.Value.String())
	})
}

// TestSecretsRedactedFromPlaintextLogs verifies redaction end-to-end: a secret property logged
// through slog never reaches the stderr log output.
//
//nolint:paralleltest // mutates global logging state and os.Stderr
func TestSecretsRedactedFromPlaintextLogs(t *testing.T) {
	prevLog, prevV, prevFlow := logging.LogToStderr, logging.Verbose, logging.LogFlow
	t.Cleanup(func() {
		logging.LogToStderr, logging.Verbose, logging.LogFlow = prevLog, prevV, prevFlow
	})

	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStderr := os.Stderr
	os.Stderr = w
	logging.InitLogging(true, 1, false)
	os.Stderr = oldStderr

	slog.Info("created resource", "props", PropertyMap{
		"name":     NewProperty("web"),
		"password": MakeSecret(NewProperty("hunter2")),
	})

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Contains(t, string(out), "[secret]")
	assert.Contains(t, string(out), "web")
	assert.NotContains(t, string(out), "hunter2")
}
