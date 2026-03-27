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

package otelreceiver

import (
	"net/url"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveFilePath(t *testing.T) {
	t.Parallel()

	t.Run("absolute path", func(t *testing.T) {
		t.Parallel()
		var (
			input    string
			expected string
		)
		if runtime.GOOS == "windows" {
			input = "file:///C:/traces.json"
			expected = `C:\traces.json`
		} else {
			input = "file:///tmp/traces.json"
			expected = "/tmp/traces.json"
		}
		path, err := resolveFilePath(input)
		require.NoError(t, err)
		require.Equal(t, expected, path)
	})

	t.Run("tilde expands to home dir", func(t *testing.T) {
		t.Parallel()
		usr, err := user.Current()
		require.NoError(t, err)

		path, err := resolveFilePath("file://~/traces.json")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(usr.HomeDir, "traces.json"), path)
	})

	t.Run("relative path is made absolute", func(t *testing.T) {
		t.Parallel()
		path, err := resolveFilePath("file://relative/path.json")
		require.NoError(t, err)
		require.True(t, filepath.IsAbs(path))
	})

	t.Run("empty path returns error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveFilePath("file://")
		require.Error(t, err)
	})
}

func TestHeadersFromQuery(t *testing.T) {
	t.Parallel()

	u, err := url.Parse("grpcs://api.honeycomb.io:443?x-some-header=abc123&other-header=banana")
	require.NoError(t, err)
	result := headersFromQuery(u.Query())
	require.NotNil(t, result)
	require.Equal(t, "abc123", result["x-some-header"])
	require.Equal(t, "banana", result["other-header"])
}

func TestNewExporterSchemes(t *testing.T) {
	t.Parallel()

	t.Run("empty endpoint returns error", func(t *testing.T) {
		t.Parallel()
		_, err := NewExporter("")
		require.Error(t, err)
	})

	t.Run("unsupported scheme returns error", func(t *testing.T) {
		t.Parallel()
		_, err := NewExporter("https://example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported endpoint scheme")
	})

	t.Run("grpc:// scheme requires a host", func(t *testing.T) {
		t.Parallel()
		_, err := NewExporter("grpc://")
		require.Error(t, err)
		require.Contains(t, err.Error(), "host is required")
	})

	t.Run("grpcs:// scheme requires a host", func(t *testing.T) {
		t.Parallel()
		_, err := NewExporter("grpcs://")
		require.Error(t, err)
		require.Contains(t, err.Error(), "host is required")
	})

	t.Run("grpc:// creates exporter", func(t *testing.T) {
		t.Parallel()
		exp, err := NewExporter("grpc://localhost:4317")
		require.NoError(t, err)
		require.NotNil(t, exp)
		require.NoError(t, exp.Shutdown(t.Context()))
	})

	t.Run("grpc:// exporter carries headers", func(t *testing.T) {
		t.Parallel()
		exp, err := NewExporter("grpc://localhost:4317?x-my-header=myvalue")
		require.NoError(t, err)
		require.NotNil(t, exp)
		grpcExp, ok := exp.(*GRPCExporter)
		require.True(t, ok, "expected *GRPCExporter")
		require.Equal(t, []string{"myvalue"}, grpcExp.headers["x-my-header"])
		_ = exp.Shutdown(t.Context())
	})

	t.Run("grpcs:// exporter carries headers", func(t *testing.T) {
		t.Parallel()
		exp, err := NewExporter("grpcs://api.example.com:443?x-api-key=testkey")
		require.NoError(t, err)
		require.NotNil(t, exp)
		grpcExp, ok := exp.(*GRPCExporter)
		require.True(t, ok, "expected *GRPCExporter")
		require.Equal(t, []string{"testkey"}, grpcExp.headers["x-api-key"])
		require.NoError(t, exp.Shutdown(t.Context()))
	})
}
