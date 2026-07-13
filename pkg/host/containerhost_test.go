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

package host

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// fakeAnalyzerServer is a minimal in-process analyzer the attach path can
// dial, standing in for a containerized pack.
type fakeAnalyzerServer struct {
	pulumirpc.UnimplementedAnalyzerServer
	cancelled atomic.Bool
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

func (s *fakeAnalyzerServer) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	s.cancelled.Store(true)
	return &emptypb.Empty{}, nil
}

func startFakeAnalyzer(t *testing.T) (*fakeAnalyzerServer, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	fake := &fakeAnalyzerServer{}
	srv := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)
	return fake, port
}

func newTestContext(t *testing.T, h plugin.Host) *plugin.Context {
	t.Helper()
	ctx, err := plugin.NewContext(t.Context(), nil, nil, h, nil, t.TempDir(), nil, false, nil)
	require.NoError(t, err)
	return ctx
}

func TestContainerHostDefersNonContainerPacks(t *testing.T) {
	t.Parallel()

	baseCalled := false
	want := plugin.NewAnalyzerWithClient("from-base", nil)
	base := &plugin.MockHost{
		PolicyAnalyzerF: func(
			ctx *plugin.Context, name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
		) (plugin.Analyzer, error) {
			baseCalled = true
			return want, nil
		},
	}
	h := NewContainerHost(base)
	ctx := newTestContext(t, h)

	// A pack directory with a non-container manifest.
	dir := t.TempDir()
	require.NoError(t, writeManifest(dir, "runtime: nodejs\n"))

	got, err := h.PolicyAnalyzer(ctx, "node-pack", dir, nil)
	require.NoError(t, err)
	assert.True(t, baseCalled)
	assert.Same(t, want, got)
}

func TestContainerHostDefersWhenManifestMissing(t *testing.T) {
	t.Parallel()

	baseCalled := false
	base := &plugin.MockHost{
		PolicyAnalyzerF: func(
			ctx *plugin.Context, name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
		) (plugin.Analyzer, error) {
			baseCalled = true
			return plugin.NewAnalyzerWithClient(name, nil), nil
		},
	}
	h := NewContainerHost(base)
	ctx := newTestContext(t, h)

	_, err := h.PolicyAnalyzer(ctx, "no-manifest", t.TempDir(), nil)
	require.NoError(t, err)
	assert.True(t, baseCalled, "manifest load failures defer to the base host's canonical error path")
}

func TestContainerHostAttachMode(t *testing.T) {
	_, port := startFakeAnalyzer(t)
	t.Setenv(plugin.EnvPolicyPackAttach, "attach-pack:"+port)

	base := &plugin.MockHost{
		PolicyAnalyzerF: func(
			ctx *plugin.Context, name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
		) (plugin.Analyzer, error) {
			t.Fatal("attach-mode packs must not reach the base host")
			return nil, nil
		},
		ServerAddrF: func() string { return "127.0.0.1:1" },
	}
	h := NewContainerHost(base)
	ctx := newTestContext(t, h)

	a, err := h.PolicyAnalyzer(ctx, "attach-pack", "", nil)
	require.NoError(t, err)

	info, err := a.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "fake-pack", info.Name)
	require.NoError(t, h.Close())
}

func TestContainerHostCachesAnalyzers(t *testing.T) {
	_, port := startFakeAnalyzer(t)
	t.Setenv(plugin.EnvPolicyPackAttach, "attach-pack:"+port)

	base := &plugin.MockHost{ServerAddrF: func() string { return "127.0.0.1:1" }}
	h := NewContainerHost(base)
	ctx := newTestContext(t, h)

	a1, err := h.PolicyAnalyzer(ctx, "attach-pack", "", nil)
	require.NoError(t, err)
	a2, err := h.PolicyAnalyzer(ctx, "attach-pack", "", nil)
	require.NoError(t, err)
	assert.Same(t, a1, a2)
	require.NoError(t, h.Close())
}

func TestContainerHostReleaseContextClosesAnalyzers(t *testing.T) {
	_, port := startFakeAnalyzer(t)
	t.Setenv(plugin.EnvPolicyPackAttach, "attach-pack:"+port)

	baseReleased := false
	base := &plugin.MockHost{
		ServerAddrF:     func() string { return "127.0.0.1:1" },
		ReleaseContextF: func(ctx *plugin.Context) error { baseReleased = true; return nil },
	}
	h := NewContainerHost(base)
	ctx := newTestContext(t, h)

	a, err := h.PolicyAnalyzer(ctx, "attach-pack", "", nil)
	require.NoError(t, err)

	require.NoError(t, h.ReleaseContext(ctx))
	assert.True(t, baseReleased)

	// The analyzer's connection is closed: RPCs now fail.
	_, err = a.GetAnalyzerInfo(t.Context())
	require.Error(t, err)

	// A fresh request boots a fresh analyzer rather than reusing the closed one.
	a2, err := h.PolicyAnalyzer(ctx, "attach-pack", "", nil)
	require.NoError(t, err)
	assert.NotSame(t, a, a2)
	require.NoError(t, h.Close())
}

func TestContainerHostSignalCancellationForwards(t *testing.T) {
	fake, port := startFakeAnalyzer(t)
	t.Setenv(plugin.EnvPolicyPackAttach, "attach-pack:"+port)

	baseSignalled := false
	base := &plugin.MockHost{
		ServerAddrF:         func() string { return "127.0.0.1:1" },
		SignalCancellationF: func() error { baseSignalled = true; return nil },
	}
	h := NewContainerHost(base)
	ctx := newTestContext(t, h)

	_, err := h.PolicyAnalyzer(ctx, "attach-pack", "", nil)
	require.NoError(t, err)

	require.NoError(t, h.SignalCancellation())
	assert.True(t, baseSignalled)
	assert.True(t, fake.cancelled.Load())
	require.NoError(t, h.Close())
}

func TestContainerHostLocalPackMissingImage(t *testing.T) {
	t.Parallel()

	h := NewContainerHost(&plugin.MockHost{})
	ctx := newTestContext(t, h)

	dir := t.TempDir()
	require.NoError(t, writeManifest(dir, "runtime:\n  name: oci\n"))

	_, err := h.PolicyAnalyzer(ctx, "oci-pack", dir, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime.options.image")
}

func writeManifest(dir, content string) error {
	return os.WriteFile(filepath.Join(dir, "PulumiPolicy.yaml"), []byte(content), 0o600)
}
