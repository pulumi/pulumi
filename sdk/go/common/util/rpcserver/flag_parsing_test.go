// Copyright 2016-2024, Pulumi Corporation.
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

package rpcserver

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

const (
	engineAddress   = "localhost:8080"
	pluginPath      = "plugin/path"
	tracingName     = "tracing-name"
	rootSpanName    = "root-span-name"
	tracingEndpoint = "localhost:9090"
	tracingFlag     = "-tracing"

	healthCheckInterval = time.Second
)

// Test the NewServer initialization with valid config
//
//nolint:paralleltest
func TestNewServer_ValidConfig(t *testing.T) {
	os.Args = []string{"cmd", "--tracing", tracingEndpoint, "--custom-flag", "yes", engineAddress, pluginPath}

	server, err := NewServer(Config{
		TracingName:         tracingName,
		RootSpanName:        rootSpanName,
		HealthcheckInterval: healthCheckInterval,
	})
	assert.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, engineAddress, server.GetEngineAddress())
	assert.Equal(t, pluginPath, server.GetPluginPath())
	assert.Equal(t, tracingEndpoint, server.GetTracing())
	assert.Equal(t, healthCheckInterval, server.getHealthcheckD())
	// assert.Equal(t, true, len(server.getGrpcOptions()) > 2) // for now
}

// Test NewServer with missing engine address (invalid config)
//
//nolint:paralleltest
func TestNewServer_MissingEngineAddress(t *testing.T) {
	// Mock os.Args (no engine address provided)
	os.Args = []string{"cmd"}

	config := Config{
		TracingName:  "test-tracing",
		RootSpanName: "test-root-span",
	}

	_, err := NewServer(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required engine RPC address argument")
}

// Test registerFlags function (private function)
//
//nolint:paralleltest
func TestServer_RegisterFlags(t *testing.T) {
	// Mock os.Args
	os.Args = []string{"cmd", "localhost:8080", tracingFlag, "test-tracing"}

	config := Config{
		TracingName:  "test-tracing",
		RootSpanName: "test-root-span",
	}

	server, err := NewServer(config)
	assert.NoError(t, err)

	// Ensure flags are set correctly
	assert.Equal(t, "test-tracing", server.GetTracing())
}

// Test getHealthcheckD returns the correct default duration
//
//nolint:paralleltest
func TestServer_GetHealthcheckD_Default(t *testing.T) {
	// Mock os.Args
	os.Args = []string{"cmd", "engineAddress"}

	config := Config{}

	server, err := NewServer(config)
	assert.NoError(t, err)

	assert.Equal(t, DefaultHealthCheck, server.getHealthcheckD())
}

// Test getHealthcheckD with custom health check duration
//
//nolint:paralleltest
func TestServer_GetHealthcheckD_Custom(t *testing.T) {
	// Mock os.Args
	os.Args = []string{"cmd", "engineAddress"}

	config := Config{
		HealthcheckInterval: 2 * time.Minute,
	}

	server, err := NewServer(config)
	assert.NoError(t, err)

	assert.Equal(t, 2*time.Minute, server.getHealthcheckD())
}

// Test SetGrpcOptions and getGrpcOptions
//
//nolint:paralleltest
func TestServer_SetGetGrpcOptions(t *testing.T) {
	// Mock os.Args
	os.Args = []string{"cmd", "engineAddress"}

	config := Config{}

	server, err := NewServer(config)
	assert.NoError(t, err)

	// Test setting custom gRPC options
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(100),
	}

	server.SetGrpcOptions(opts)

	// Test getting gRPC options
	retOpts := server.getGrpcOptions()
	assert.Equal(t, opts, retOpts)
}

type runParams struct {
	tracing       string
	engineAddress string
	root          string
}

// Test from pulumi-language-go
//
//nolint:paralleltest
func TestParseRunParams(t *testing.T) {
	tests := []struct {
		desc          string
		give          []string
		want          runParams
		wantErr       string // non-empty if we expect an error
		wantErrServer string // non-empty if we expect an error
	}{
		{
			desc:          "no arguments",
			wantErrServer: "missing required engine RPC address argument",
		},
		{
			desc: "no options",
			give: []string{"cmd", "localhost:1234"},
			want: runParams{
				engineAddress: "localhost:1234",
			},
		},
		{
			desc: "tracing",
			give: []string{"cmd", "-tracing", "foo.trace", "localhost:1234"},
			want: runParams{
				tracing:       "foo.trace",
				engineAddress: "localhost:1234",
			},
		},
		{
			desc: "binary",
			give: []string{"cmd", "-binary", "foo", "localhost:1234"},
			want: runParams{
				engineAddress: "localhost:1234",
			},
		},
		{
			desc: "buildTarget",
			give: []string{"cmd", "-buildTarget", "foo", "localhost:1234"},
			want: runParams{
				engineAddress: "localhost:1234",
			},
		},
		{
			desc: "root",
			give: []string{"cmd", "-root", "path/to/root", "localhost:1234"},
			want: runParams{
				engineAddress: "localhost:1234",
				root:          "path/to/root",
			},
		},
		{
			desc:    "unknown option",
			give:    []string{"cmd", "-unknown-option", "bar", "localhost:1234"},
			wantErr: "flag provided but not defined: -unknown-option",
		},
	}

	// test cases depends on os.Args
	os.Args = []string{}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			// Use a FlagSet with ContinueOnError for each case
			// instead of using the global flag set.
			//
			// The global flag set uses flag.ExitOnError,
			// so it cannot validate error cases during tests.
			fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			fset.SetOutput(iotest.LogWriter(t))

			config := Config{Flag: fset, Args: tt.give}
			server, err := NewServer(config)
			if tt.wantErrServer != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			if server == nil {
				t.Fatal("nil server")
				return
			}

			if tt.want.tracing != "" {
				assert.Equal(t, tt.want.tracing, server.GetTracing())
			}
			if tt.want.engineAddress != "" {
				assert.Equal(t, tt.want.engineAddress, server.GetEngineAddress())
			}

			server.Flag.String("binary", "", "[obsolete] Look on path for a binary executable with this name")
			server.Flag.String("buildTarget", "", "[obsolete] Path to use to output the compiled Pulumi Go program")
			root := server.Flag.String("root", "", "[obsolete] Project root path to use")

			err = server.Flag.Parse(tt.give[1:])
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)

			if tt.want.root != "" {
				assert.Equal(t, tt.want.root, *root)
			}
		})
	}
}
