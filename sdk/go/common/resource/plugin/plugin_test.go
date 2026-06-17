// Copyright 2016, Pulumi Corporation.
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
	"fmt"
	"net"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func TestPrematureExit(t *testing.T) {
	t.Parallel()

	// Start a gRPC server to simulate a provider plugin.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	pulumirpc.RegisterResourceProviderServer(server, &pulumirpc.UnimplementedResourceProviderServer{})

	ready := make(chan struct{})
	go func() {
		close(ready)
		server.Serve(listener) //nolint:errcheck
	}()
	<-ready

	port := listener.Addr().(*net.TCPAddr).Port
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	sink := &diag.MockSink{}

	plug := &Plugin{
		Bin:  "test-plugin",
		Conn: conn,
		Kill: func() error { return nil },
		unstructuredOutput: &unstructuredOutput{
			diag: sink,
		},
	}
	plug.unstructuredOutput.output.WriteString("some plugin output\n")

	prov := &provider{
		plug:      plug,
		clientRaw: pulumirpc.NewResourceProviderClient(conn),
	}

	// Simulate a crash: stop the server before calling Close.
	server.Stop()

	err = prov.Close()
	require.NoError(t, err)

	// Verify the "exited prematurely" error was reported.
	require.Len(t, sink.Messages[diag.Error], 1)
	msg := sink.Messages[diag.Error][0].Diag.Message
	require.Contains(t, msg, "exited prematurely")
	require.Contains(t, msg, "some plugin output")
}

func TestCheckVersionRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		cliVersion         string
		pulumiVersionRange string
		expectedError      string
	}{
		{
			name:               "exact match",
			cliVersion:         "3.0.0",
			pulumiVersionRange: "3.0.0",
		},
		{
			name:               "greater than",
			cliVersion:         "3.1.0",
			pulumiVersionRange: ">=3.0.0",
		},
		{
			name:               "within range",
			cliVersion:         "3.5.0",
			pulumiVersionRange: ">=3.0.0 <4.0.0",
		},
		{
			name:               "too old",
			cliVersion:         "2.9.0",
			pulumiVersionRange: ">=3.0.0",
			//nolint:lll
			expectedError: "CLI version 2.9.0 does not satisfy the version range \">=3.0.0\"",
		},
		{
			name:               "too new",
			cliVersion:         "4.0.0",
			pulumiVersionRange: "<4.0.0",
			//nolint:lll
			expectedError: "CLI version 4.0.0 does not satisfy the version range \"<4.0.0\"",
		},
		{
			name:               "exclude",
			cliVersion:         "3.1.0",
			pulumiVersionRange: ">=3.0.0 !3.1.0",
			//nolint:lll
			expectedError: "CLI version 3.1.0 does not satisfy the version range \">=3.0.0 !3.1.0\"",
		},
		{
			name:               "exclude 2",
			cliVersion:         "3.1.1",
			pulumiVersionRange: ">=3.0.0 !3.1.0",
		},
		{
			name:               "exclude 3",
			cliVersion:         "3.0.1",
			pulumiVersionRange: ">=3.0.0 !3.1.0",
		},
		{
			name:               "no range",
			cliVersion:         "1.0.0",
			pulumiVersionRange: "",
		},
		{
			name:               "no cli version",
			cliVersion:         "",
			pulumiVersionRange: "1.2.3",
		},
		{
			name:               "cli dev version ok",
			cliVersion:         "3.215.0-alpha.x75fc436",
			pulumiVersionRange: ">=3.214.0",
		},
		{
			name:               "cli dev version bad",
			cliVersion:         "3.215.0-alpha.x75fc436",
			pulumiVersionRange: ">=3.215.0",
			//nolint:lll
			expectedError: "CLI version 3.215.0-alpha.x75fc436 does not satisfy the version range \">=3.215.0\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePulumiVersionRange(tt.pulumiVersionRange, tt.cliVersion)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestResourceProviderEnvVars(t *testing.T) {
	t.Parallel()

	ctx := &Context{ResourceProviderEnv: map[string]string{"PULUMI_ACCESS_TOKEN": "secret"}}

	t.Run("env is injected", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"PULUMI_ACCESS_TOKEN=secret"}, resourceProviderEnvVars(ctx))
	})

	t.Run("nil context is safe", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, resourceProviderEnvVars(nil))
	})
}
