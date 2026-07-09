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
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

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

func (s *fakeAnalyzerServer) Handshake(
	ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (s *fakeAnalyzerServer) GetAnalyzerInfo(
	ctx context.Context, req *emptypb.Empty,
) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{Name: "fake-pack", Version: "1.0.0"}, nil
}

// fakeHost implements only what the OCI/attach boot paths call.
type fakeHost struct {
	Host
	addr string
}

func (h *fakeHost) ServerAddr() string                 { return h.addr }
func (h *fakeHost) AttachDebugger(spec DebugSpec) bool { return false }

func TestNewPolicyAnalyzerAttachMode(t *testing.T) {
	addr := startFakeAnalyzer(t)
	_, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	t.Setenv(EnvPolicyPackAttach, "attach-pack:"+portStr)

	ctx, err := NewContext(t.Context(), nil, nil, &MockHost{}, nil, t.TempDir(), nil, false, nil)
	require.NoError(t, err)

	a, err := NewPolicyAnalyzer(&fakeHost{addr: "127.0.0.1:1"}, ctx, "attach-pack",
		t.TempDir() /* no manifest needed: attach short-circuits */, nil, nil)
	require.NoError(t, err)

	info, err := a.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "fake-pack", info.Name)
	require.NoError(t, a.Close())
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
