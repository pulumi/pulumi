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
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestGetPolicyPackAttachPort(t *testing.T) {
	t.Setenv(EnvPolicyPackAttach, "security:1234,cost-controls:5678")

	port, err := GetPolicyPackAttachPort("security")
	require.NoError(t, err)
	require.NotNil(t, port)
	assert.Equal(t, 1234, *port)

	port, err = GetPolicyPackAttachPort("cost-controls")
	require.NoError(t, err)
	require.NotNil(t, port)
	assert.Equal(t, 5678, *port)

	port, err = GetPolicyPackAttachPort("unlisted")
	require.NoError(t, err)
	assert.Nil(t, port)
}

func TestGetPolicyPackAttachPortBadPort(t *testing.T) {
	t.Setenv(EnvPolicyPackAttach, "security:not-a-port")
	_, err := GetPolicyPackAttachPort("security")
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvPolicyPackAttach)
}

func TestGetPolicyPackAttachPortUnset(t *testing.T) {
	t.Parallel()
	port, err := GetPolicyPackAttachPort("security")
	require.NoError(t, err)
	assert.Nil(t, port)
}

// fakeAnalyzerServer is a minimal in-process analyzer gRPC server.
type fakeAnalyzerServer struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func startFakeAnalyzer(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(srv, &fakeAnalyzerServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func TestDialAnalyzerWithRetrySucceeds(t *testing.T) {
	t.Parallel()
	addr := startFakeAnalyzer(t)
	conn, err := dialAnalyzerWithRetry(t.Context(), addr, 5*time.Second, nil)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestDialAnalyzerWithRetryTimesOut(t *testing.T) {
	t.Parallel()
	// A port with nothing listening.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	_, err = dialAnalyzerWithRetry(t.Context(), addr, 500*time.Millisecond, nil)
	require.Error(t, err)
}

func TestDialAnalyzerWithRetryFailsFastOnExit(t *testing.T) {
	t.Parallel()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	start := time.Now()
	_, err = dialAnalyzerWithRetry(t.Context(), addr, 30*time.Second,
		func() (bool, string) { return false, "container crashed: OOM" })
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%v", err), "container crashed: OOM")
	assert.Less(t, time.Since(start), 10*time.Second, "should fail fast, not wait out the timeout")
}
