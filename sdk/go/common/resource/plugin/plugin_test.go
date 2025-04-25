// Copyright 2016-2021, Pulumi Corporation.
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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestLogFlowArgumentPropagation(t *testing.T) {
	t.Parallel()

	engine := "127.0.0.1:12345"

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs: []string{engine},
	}), []string{engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs: []string{engine},
		logFlow:    true,
		verbose:    9,
	}), []string{"-v=9", engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs:  []string{engine},
		logFlow:     true,
		logToStderr: true,
		verbose:     9,
	}), []string{"--logtostderr", "-v=9", engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs:      []string{engine},
		tracingEndpoint: "127.0.0.1:6007",
	}), []string{"--tracing", "127.0.0.1:6007", engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs:      []string{engine},
		logFlow:         true,
		logToStderr:     true,
		verbose:         9,
		tracingEndpoint: "127.0.0.1:6007",
	}), []string{"--logtostderr", "-v=9", "--tracing", "127.0.0.1:6007", "127.0.0.1:12345"})
}

func TestParsePort(t *testing.T) {
	t.Parallel()

	for _, port := range []string{
		"1234",
		" 1234",
		"     1234",
		"1234 ",
		"1234     ",
		"1234\r\n",
		"1234\n",
		"\x1b]9;4;3;\x1b\\\x1b]9;4;0;\x1b\\1234",
		"\x1b]9;4;3;\x1b\\\x1b]9;4;0;\x1b\\ 1234",
		"\x1b]9;4;3;\x1b\\\x1b]9;4;0;\x1b\\ 1234 ",
		"\x1b]9;4;3;\x1b\\\x1b]9;4;0;\x1b\\1234\n",
	} {
		parsedPort, err := parsePort(port)
		require.NoError(t, err)
		require.Equal(t, 1234, parsedPort)
	}

	for _, port := range []string{
		"",
		"banana",
		"0",
		"-1234",
		"100000",
	} {
		_, err := parsePort(port)
		require.Error(t, err)
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	startServer := func(healthService bool) (*grpc.Server, *plugin) {
		listener, _ := net.Listen("tcp", "127.0.0.1:0")
		server := grpc.NewServer()

		if healthService {
			healthServer := health.NewServer()
			grpc_health_v1.RegisterHealthServer(server, healthServer)
		}

		ready := make(chan struct{})
		go func() {
			close(ready) // Signal that server is ready
			err := server.Serve(listener)
			require.NoError(t, err)
		}()
		<-ready // Wait until the server is ready before continuing

		port := listener.Addr().(*net.TCPAddr).Port

		type foo struct{}
		handshake := func(context.Context, string, string, *grpc.ClientConn) (*foo, error) {
			return &foo{}, nil
		}

		conn, _, err := dialPlugin(port, "test", "test", handshake, []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		})
		require.NoError(t, err)

		return server, &plugin{Conn: conn}
	}

	tests := []struct {
		name           string
		healthService  bool
		shutdownServer bool
		expected       bool
	}{
		{
			name:           "Server with health check - running",
			healthService:  true,
			shutdownServer: false,
			expected:       true,
		},
		{
			name:           "Server with health check - crashed",
			healthService:  true,
			shutdownServer: true,
			expected:       false,
		},
		{
			name:           "Server without health check - running",
			healthService:  false,
			shutdownServer: false,
			expected:       true,
		},
		{
			name:           "Server without health check - crashed",
			healthService:  false,
			shutdownServer: true,
			expected:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, p := startServer(tt.healthService)

			// Simulate a crash by stopping the server before calling healthCheck.
			if tt.shutdownServer {
				server.Stop()
				// Give time for cleanup
				time.Sleep(100 * time.Millisecond)
			}

			result := p.healthCheck()
			assert.Equal(t, tt.expected, result)

			p.Conn.Close()
			server.Stop()
		})
	}
}

func TestStartupFailure(t *testing.T) {
	d := diagtest.LogSink(t)
	ctx, err := NewContext(d, d, nil, nil, "", nil, false, nil)
	require.NoError(t, err)

	pluginPath, err := filepath.Abs("./testdata/language")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Check exec.LookPath finds the analyzer
	file, err := exec.LookPath("pulumi-language-test")
	require.NoError(t, err)
	require.Contains(t, file, "pulumi-language-test")

	_, err = NewProviderFromPath(ctx.Host, ctx, filepath.Join("testdata", "test-plugin", "test-plugin"))
	require.ErrorContains(t, err, "could not read plugin [testdata/test-plugin/test-plugin]: not implemented")
}
