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

package sdk

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockPolicyPackServer is a minimal gRPC AnalyzerServer implementation used for testing PolicyProxy.
// By default (embedding UnimplementedAnalyzerServer) every RPC returns codes.Unimplemented.
// Individual test cases override the methods they care about.
type mockPolicyPackServer struct {
	pulumirpc.UnimplementedAnalyzerServer

	// configureStackFn, if non-nil, is called by ConfigureStack. When nil the embedded
	// UnimplementedAnalyzerServer.ConfigureStack is used, which returns codes.Unimplemented —
	// exactly what old policy-pack SDKs that predate ConfigureStack support do.
	configureStackFn func(context.Context, *pulumirpc.AnalyzerStackConfigureRequest) (
		*pulumirpc.AnalyzerStackConfigureResponse, error)
}

func (m *mockPolicyPackServer) GetPluginInfo(
	_ context.Context, _ *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "1.0.0"}, nil
}

func (m *mockPolicyPackServer) ConfigureStack(
	ctx context.Context,
	req *pulumirpc.AnalyzerStackConfigureRequest,
) (*pulumirpc.AnalyzerStackConfigureResponse, error) {
	if m.configureStackFn != nil {
		return m.configureStackFn(ctx, req)
	}
	// Delegate to the embedded stub, which returns codes.Unimplemented.
	return m.UnimplementedAnalyzerServer.ConfigureStack(ctx, req)
}

// startMockPolicyPackServer starts a gRPC server with srv registered as the AnalyzerServer and
// returns the port it is listening on. The server is stopped when t ends.
func startMockPolicyPackServer(t *testing.T, srv *mockPolicyPackServer) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcSrv := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(grpcSrv, srv)
	go grpcSrv.Serve(lis) //nolint:errcheck
	t.Cleanup(grpcSrv.Stop)

	return lis.Addr().(*net.TCPAddr).Port
}

// startLongRunningCmd starts a process that stays alive until the test ends (or it is killed).
func startLongRunningCmd(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "100")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill() //nolint:errcheck
		}
	})
	return cmd
}

// runAttach runs proxy.Attach in a goroutine and returns a channel that receives the result.
func runAttach(ctx context.Context, proxy *PolicyProxy, cmd *exec.Cmd) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- proxy.Attach(ctx, cmd) }()
	return ch
}

// awaitClientFulfilled waits up to 5 seconds for proxy.client to be resolved and returns it.
func awaitClientFulfilled(t *testing.T, proxy *PolicyProxy) pulumirpc.AnalyzerClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := proxy.client.Promise().Result(ctx)
	require.NoError(t, err, "proxy.client promise should be fulfilled without error")
	require.NotNil(t, client)
	return client
}

// TestPolicyProxy_Attach_OldPolicyPackNoConfigureStack tests the fix for issue #21864.
//
// Old policy-pack SDKs (e.g. @pulumi/policy <= v1.18.1) do not implement the ConfigureStack RPC.
// Before the fix, PolicyProxy.Attach returned an error without fulfilling the client promise,
// causing awaitClient (and thus GetPluginInfo / GetAnalyzerInfo) to block forever, which in turn
// prevented GracefulStop from completing and hung the engine.
func TestPolicyProxy_Attach_OldPolicyPackNoConfigureStack(t *testing.T) {
	t.Parallel()

	// This mock relies on the default UnimplementedAnalyzerServer.ConfigureStack, which returns
	// codes.Unimplemented — just like an old policy pack that has no knowledge of this RPC.
	port := startMockPolicyPackServer(t, &mockPolicyPackServer{})

	ctx := context.Background()
	proxy, stdoutW, err := NewPolicyProxy(ctx, &bytes.Buffer{})
	require.NoError(t, err)

	// Simulate the engine calling ConfigureStack on the proxy before the policy pack starts.
	_, err = proxy.ConfigureStack(ctx, &pulumirpc.AnalyzerStackConfigureRequest{})
	require.NoError(t, err)

	cmd := startLongRunningCmd(t)
	attachDone := runAttach(ctx, proxy, cmd)

	// Give Attach the mock server port via the pipe. This must happen after runAttach because
	// io.Pipe is synchronous: the write blocks until Attach's Fscanf reads from the other end.
	fmt.Fprintf(stdoutW, "%d\n", port)

	// Before the fix this would time out because client was never fulfilled.
	client := awaitClientFulfilled(t, proxy)

	// Sanity-check: the fulfilled client is usable.
	info, err := client.GetPluginInfo(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.GetVersion())

	// Kill the subprocess to let Attach return.
	cmd.Process.Kill() //nolint:errcheck
	<-attachDone       // wait for goroutine to exit; error is expected (killed process)
}

// TestPolicyProxy_Attach_NewPolicyPackWithConfigureStack verifies that when the policy pack
// implements ConfigureStack successfully the proxy client is still fulfilled.
func TestPolicyProxy_Attach_NewPolicyPackWithConfigureStack(t *testing.T) {
	t.Parallel()

	port := startMockPolicyPackServer(t, &mockPolicyPackServer{
		configureStackFn: func(_ context.Context, _ *pulumirpc.AnalyzerStackConfigureRequest) (
			*pulumirpc.AnalyzerStackConfigureResponse, error,
		) {
			return &pulumirpc.AnalyzerStackConfigureResponse{}, nil
		},
	})

	ctx := context.Background()
	proxy, stdoutW, err := NewPolicyProxy(ctx, &bytes.Buffer{})
	require.NoError(t, err)

	_, err = proxy.ConfigureStack(ctx, &pulumirpc.AnalyzerStackConfigureRequest{})
	require.NoError(t, err)

	cmd := startLongRunningCmd(t)
	attachDone := runAttach(ctx, proxy, cmd)

	// Write port after starting Attach goroutine so the synchronous pipe write doesn't block.
	fmt.Fprintf(stdoutW, "%d\n", port)

	client := awaitClientFulfilled(t, proxy)

	info, err := client.GetPluginInfo(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.GetVersion())

	cmd.Process.Kill() //nolint:errcheck
	<-attachDone
}

// TestPolicyProxy_Attach_ConfigureStackError verifies that when ConfigureStack fails with a
// non-Unimplemented error the client promise is rejected (not left unresolved), so callers of
// awaitClient get an error rather than hanging indefinitely.
func TestPolicyProxy_Attach_ConfigureStackError(t *testing.T) {
	t.Parallel()

	configureErr := status.Error(codes.Internal, "policy pack misconfigured")
	port := startMockPolicyPackServer(t, &mockPolicyPackServer{
		configureStackFn: func(_ context.Context, _ *pulumirpc.AnalyzerStackConfigureRequest) (
			*pulumirpc.AnalyzerStackConfigureResponse, error,
		) {
			return nil, configureErr
		},
	})

	ctx := context.Background()
	proxy, stdoutW, err := NewPolicyProxy(ctx, &bytes.Buffer{})
	require.NoError(t, err)

	_, err = proxy.ConfigureStack(ctx, &pulumirpc.AnalyzerStackConfigureRequest{})
	require.NoError(t, err)

	// We don't need a long-running cmd here: Attach returns early on ConfigureStack error before
	// waiting for the process to exit. Use a no-op process so cmd.Wait() is valid.
	cmd := exec.Command("true")
	require.NoError(t, cmd.Start())
	attachDone := runAttach(ctx, proxy, cmd)

	// Write port after starting Attach goroutine so the synchronous pipe write doesn't block.
	fmt.Fprintf(stdoutW, "%d\n", port)

	// Attach should return an error.
	select {
	case attachErr := <-attachDone:
		require.ErrorContains(t, attachErr, "policy pack configuration failed")
	case <-time.After(5 * time.Second):
		t.Fatal("Attach did not return within timeout after ConfigureStack error")
	}

	// The client promise should be rejected, not unresolved.
	clientCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, clientErr := proxy.client.Promise().Result(clientCtx)
	require.ErrorContains(t, clientErr, "policy pack configuration failed")
}
