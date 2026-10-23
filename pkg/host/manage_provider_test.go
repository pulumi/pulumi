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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// When set, the test binary serves cancelLoggingProvider instead of running tests, so that tests can start it as a
// provider plugin through NewProviderFromPath. The value is the file each Cancel RPC is logged to.
const cancelLogEnv = "PULUMI_HOST_TEST_CANCEL_LOG"

func TestMain(m *testing.M) {
	if path := os.Getenv(cancelLogEnv); path != "" {
		if err := serveCancelLoggingProvider(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	os.Exit(m.Run())
}

type cancelLoggingProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	path string
}

func (p *cancelLoggingProvider) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	f, err := os.OpenFile(p.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, err = f.WriteString("cancel\n")
	return &emptypb.Empty{}, err
}

func serveCancelLoggingProvider(path string) error {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, &cancelLoggingProvider{path: path})
			return nil
		},
	})
	if err != nil {
		return err
	}
	fmt.Println(handle.Port)
	return <-handle.Done
}

// startPathProvider starts the test binary as a provider through NewProviderFromPath and returns it with the file
// its Cancel RPCs are logged to.
func startPathProvider(t *testing.T, host *defaultHost, ctx *plugin.Context) (plugin.Provider, string) {
	cancelLog := filepath.Join(t.TempDir(), "cancels")
	t.Setenv(cancelLogEnv, cancelLog)
	bin, err := os.Executable()
	require.NoError(t, err)
	prov, err := plugin.NewProviderFromPath(host, ctx, bin)
	require.NoError(t, err)
	require.Len(t, host.resourcePlugins, 1)
	return prov, cancelLog
}

func cancelCount(t *testing.T, cancelLog string) int {
	data, err := os.ReadFile(cancelLog)
	require.NoError(t, err)
	return strings.Count(string(data), "cancel\n")
}

//nolint:paralleltest // startPathProvider calls t.Setenv.
func TestNewProviderFromPathClosedWithContext(t *testing.T) {
	host, ctx := newManagingHost(t)
	_, cancelLog := startPathProvider(t, host, ctx)

	require.NoError(t, ctx.Close())
	assert.Empty(t, host.resourcePlugins)
	assert.Equal(t, 1, cancelCount(t, cancelLog))
}

//nolint:paralleltest // startPathProvider calls t.Setenv.
func TestNewProviderFromPathClosedThroughHost(t *testing.T) {
	host, ctx := newManagingHost(t)
	prov, cancelLog := startPathProvider(t, host, ctx)

	require.NoError(t, prov.Close())
	assert.Empty(t, host.resourcePlugins)
	assert.Equal(t, 1, cancelCount(t, cancelLog))
}

type closeCountingProvider struct {
	plugin.Provider

	closes atomic.Int32
}

func (p *closeCountingProvider) SignalCancellation(context.Context) error { return nil }

func (p *closeCountingProvider) Close() error {
	p.closes.Add(1)
	return nil
}

func newManagingHost(t *testing.T) (*defaultHost, *plugin.Context) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)
	host, ok := h.(*defaultHost)
	require.True(t, ok)
	return host, ctx
}

func TestManageProviderClosedWithContext(t *testing.T) {
	t.Parallel()

	host, ctx := newManagingHost(t)
	prov := &closeCountingProvider{}
	_, err := host.ManageProvider(ctx, prov, "path/to/provider")
	require.NoError(t, err)

	require.NoError(t, ctx.Close())
	assert.Equal(t, int32(1), prov.closes.Load())
	assert.Empty(t, host.resourcePlugins)
}

func TestManageProviderClosedWithHost(t *testing.T) {
	t.Parallel()

	host, ctx := newManagingHost(t)
	prov := &closeCountingProvider{}
	_, err := host.ManageProvider(ctx, prov, "path/to/provider")
	require.NoError(t, err)

	require.NoError(t, host.Close())
	assert.Equal(t, int32(1), prov.closes.Load())
	require.NoError(t, ctx.Close())
}

func TestManageProviderClosedOnce(t *testing.T) {
	t.Parallel()

	host, ctx := newManagingHost(t)
	prov := &closeCountingProvider{}
	managed, err := host.ManageProvider(ctx, prov, "path/to/provider")
	require.NoError(t, err)

	require.NoError(t, managed.Close())
	assert.Empty(t, host.resourcePlugins)
	require.NoError(t, ctx.Close())
	require.NoError(t, host.Close())
	assert.Equal(t, int32(1), prov.closes.Load())
}

func TestManageProviderAfterHostClose(t *testing.T) {
	t.Parallel()

	host, ctx := newManagingHost(t)
	require.NoError(t, host.Close())

	prov := &closeCountingProvider{}
	_, err := host.ManageProvider(ctx, prov, "path/to/provider")
	require.ErrorContains(t, err, "plugin host is shutting down")
	assert.Equal(t, int32(1), prov.closes.Load())
}
