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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

const (
	engineAddress   = "localhost:8080"
	pluginPath      = "plugin/path"
	tracingName     = "tracing-name"
	rootSpanName    = "root-span-name"
	tracingEndpoint = "localhost:9090"
	tracingFlag     = "--tracing"

	healthCheckInterval = time.Second
)

// Test the NewServer initialization with valid config
func TestNewServer_ValidConfig(t *testing.T) {
	t.Parallel()
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
func TestNewServer_MissingEngineAddress(t *testing.T) {
	t.Parallel()
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
func TestServer_RegisterFlags(t *testing.T) {
	t.Parallel()
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
func TestServer_GetHealthcheckD_Default(t *testing.T) {
	t.Parallel()
	// Mock os.Args
	os.Args = []string{"cmd", "engineAddress"}

	config := Config{}

	server, err := NewServer(config)
	assert.NoError(t, err)

	assert.Equal(t, DefaultHealthCheck, server.getHealthcheckD())
}

// Test getHealthcheckD with custom health check duration
func TestServer_GetHealthcheckD_Custom(t *testing.T) {
	t.Parallel()
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
func TestServer_SetGetGrpcOptions(t *testing.T) {
	t.Parallel()
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
