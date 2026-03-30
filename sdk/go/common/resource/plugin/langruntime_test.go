// Copyright 2024, Pulumi Corporation.
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
	"errors"
	"io"
	"sync"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestMakeExecutablePromptChoices(t *testing.T) {
	t.Parallel()

	// Not found executables come after the found ones, and have a [not found] suffix.
	choices := MakeExecutablePromptChoices("executable_that_does_not_exist_in_path", "ls")
	require.Len(t, choices, 2)
	require.Equal(t, choices[0].StringValue, "ls")
	require.Equal(t, choices[0].DisplayName, "ls")
	require.Equal(t, choices[1].StringValue, "executable_that_does_not_exist_in_path")
	require.Equal(t, choices[1].DisplayName, "executable_that_does_not_exist_in_path [not found]")

	// Executables are not reordered within their group.
	choices = MakeExecutablePromptChoices("ls", "cat", "zzz_does_not_exist", "aaa_does_not_exist")
	require.Equal(t, choices[0].StringValue, "ls")
	require.Equal(t, choices[0].DisplayName, "ls")
	require.Equal(t, choices[1].StringValue, "cat")
	require.Equal(t, choices[1].DisplayName, "cat")
	require.Equal(t, choices[2].StringValue, "zzz_does_not_exist")
	require.Equal(t, choices[2].DisplayName, "zzz_does_not_exist [not found]")
	require.Equal(t, choices[3].StringValue, "aaa_does_not_exist")
	require.Equal(t, choices[3].DisplayName, "aaa_does_not_exist [not found]")
}

type MockLanguageRuntimeClient struct {
	RunPluginF func(ctx context.Context, info *pulumirpc.RunPluginRequest,
	) (pulumirpc.LanguageRuntime_RunPluginClient, error)

	RunPlugin2F func(ctx context.Context,
	) (pulumirpc.LanguageRuntime_RunPlugin2Client, error)
}

func (m *MockLanguageRuntimeClient) RunPlugin(
	ctx context.Context, info *pulumirpc.RunPluginRequest, _ ...grpc.CallOption,
) (pulumirpc.LanguageRuntime_RunPluginClient, error) {
	return m.RunPluginF(ctx, info)
}

func (m *MockLanguageRuntimeClient) RunPlugin2(
	ctx context.Context, opts ...grpc.CallOption,
) (pulumirpc.LanguageRuntime_RunPlugin2Client, error) {
	if m.RunPlugin2F != nil {
		return m.RunPlugin2F(ctx)
	}
	return nil, status.Errorf(codes.Unimplemented, "RunPlugin2 not implemented")
}

func (m *MockLanguageRuntimeClient) GetRequiredPackages(
	ctx context.Context, in *pulumirpc.GetRequiredPackagesRequest, opts ...grpc.CallOption,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) GetRequiredPlugins(
	ctx context.Context, in *pulumirpc.GetRequiredPluginsRequest, opts ...grpc.CallOption,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) Run(
	ctx context.Context, in *pulumirpc.RunRequest, opts ...grpc.CallOption,
) (*pulumirpc.RunResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) GetPluginInfo(
	ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption,
) (*pulumirpc.PluginInfo, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) InstallDependencies(
	ctx context.Context, in *pulumirpc.InstallDependenciesRequest, opts ...grpc.CallOption,
) (pulumirpc.LanguageRuntime_InstallDependenciesClient, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) RuntimeOptionsPrompts(
	ctx context.Context, in *pulumirpc.RuntimeOptionsRequest, opts ...grpc.CallOption,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) Template(
	ctx context.Context, in *pulumirpc.TemplateRequest, opts ...grpc.CallOption,
) (*pulumirpc.TemplateResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) About(
	ctx context.Context, in *pulumirpc.AboutRequest, opts ...grpc.CallOption,
) (*pulumirpc.AboutResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) GetProgramDependencies(
	ctx context.Context, in *pulumirpc.GetProgramDependenciesRequest, opts ...grpc.CallOption,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) GenerateProgram(
	ctx context.Context, in *pulumirpc.GenerateProgramRequest, opts ...grpc.CallOption,
) (*pulumirpc.GenerateProgramResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) GenerateProject(
	ctx context.Context, in *pulumirpc.GenerateProjectRequest, opts ...grpc.CallOption,
) (*pulumirpc.GenerateProjectResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) GeneratePackage(
	ctx context.Context, in *pulumirpc.GeneratePackageRequest, opts ...grpc.CallOption,
) (*pulumirpc.GeneratePackageResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) Pack(
	ctx context.Context, in *pulumirpc.PackRequest, opts ...grpc.CallOption,
) (*pulumirpc.PackResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) Link(
	ctx context.Context, in *pulumirpc.LinkRequest, opts ...grpc.CallOption,
) (*pulumirpc.LinkResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) Handshake(
	ctx context.Context, req *pulumirpc.LanguageHandshakeRequest, opts ...grpc.CallOption,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	panic("not implemented")
}

func (m *MockLanguageRuntimeClient) Cancel(
	ctx context.Context, req *emptypb.Empty, opts ...grpc.CallOption,
) (*emptypb.Empty, error) {
	panic("not implemented")
}

func TestRunPluginPassesCorrectPwd(t *testing.T) {
	t.Parallel()

	returnErr := errors.New("erroring so we don't need to implement the whole thing")
	mockLanguageRuntime := &MockLanguageRuntimeClient{
		RunPluginF: func(
			ctx context.Context, info *pulumirpc.RunPluginRequest,
		) (pulumirpc.LanguageRuntime_RunPluginClient, error) {
			require.Equal(t, "/tmp", info.Pwd)
			return nil, returnErr
		},
	}

	pCtx, err := NewContext(t.Context(), nil, nil, nil, nil, "", nil, false, nil, nil)
	require.NoError(t, err)
	host := &langhost{
		ctx:     pCtx,
		runtime: "go",
		plug:    nil,
		client:  mockLanguageRuntime,
	}

	// Test that the plugin is run with the correct working directory.
	_, _, _, _, err = host.RunPlugin(pCtx.Request(), RunPluginInfo{
		WorkingDirectory: "/tmp",
	})
	require.Equal(t, returnErr, err)
}

func TestRunPluginPassesLoaderAddress(t *testing.T) {
	t.Parallel()

	const expectedLoaderAddr = "127.0.0.1:12345"

	mockLanguageRuntime := &MockLanguageRuntimeClient{
		RunPluginF: func(
			ctx context.Context, info *pulumirpc.RunPluginRequest,
		) (pulumirpc.LanguageRuntime_RunPluginClient, error) {
			require.Equal(t, expectedLoaderAddr, info.LoaderTarget)
			return nil, assert.AnError
		},
	}

	mockHost := &MockHost{
		LoaderAddrF: func() string {
			return expectedLoaderAddr
		},
	}

	pCtx, err := NewContext(t.Context(), nil, nil, mockHost, nil, "", nil, false, nil, nil)
	require.NoError(t, err)

	host := &langhost{
		ctx:     pCtx,
		runtime: "test",
		plug:    nil,
		client:  mockLanguageRuntime,
	}

	_, _, _, _, err = host.RunPlugin(pCtx.Request(), RunPluginInfo{
		WorkingDirectory: "/tmp",
		LoaderAddress:    expectedLoaderAddr,
	})
	require.ErrorIs(t, err, assert.AnError)
}

type mockRunPlugin2Stream struct {
	mu     sync.Mutex
	sent   []*pulumirpc.RunPlugin2Request
	recvCh chan *pulumirpc.RunPluginResponse
	ctx    context.Context
	cancel context.CancelFunc
}

func newMockRunPlugin2Stream(ctx context.Context) *mockRunPlugin2Stream {
	ctx, cancel := context.WithCancel(ctx)
	return &mockRunPlugin2Stream{
		recvCh: make(chan *pulumirpc.RunPluginResponse, 100),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *mockRunPlugin2Stream) Send(req *pulumirpc.RunPlugin2Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, req)
	return nil
}

func (s *mockRunPlugin2Stream) Recv() (*pulumirpc.RunPluginResponse, error) {
	select {
	case msg, ok := <-s.recvCh:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *mockRunPlugin2Stream) Header() (metadata.MD, error) { return nil, nil }
func (s *mockRunPlugin2Stream) Trailer() metadata.MD         { return nil }
func (s *mockRunPlugin2Stream) CloseSend() error             { return nil }
func (s *mockRunPlugin2Stream) Context() context.Context     { return s.ctx }
func (s *mockRunPlugin2Stream) SendMsg(any) error            { return nil }
func (s *mockRunPlugin2Stream) RecvMsg(any) error            { return nil }

func (s *mockRunPlugin2Stream) sentRequests() []*pulumirpc.RunPlugin2Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*pulumirpc.RunPlugin2Request{}, s.sent...)
}

func TestRunPlugin2PassesCorrectFields(t *testing.T) {
	t.Parallel()

	stream := newMockRunPlugin2Stream(t.Context())
	mockClient := &MockLanguageRuntimeClient{
		RunPlugin2F: func(ctx context.Context) (pulumirpc.LanguageRuntime_RunPlugin2Client, error) {
			return stream, nil
		},
	}

	pCtx, err := NewContext(t.Context(), nil, nil, nil, nil, "", nil, false, nil, nil)
	require.NoError(t, err)
	host := &langhost{ctx: pCtx, runtime: "test", client: mockClient}

	stream.recvCh <- &pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: 0},
	}
	close(stream.recvCh)

	stdout, stderr, cancel, done, err := host.RunPlugin(pCtx.Request(), RunPluginInfo{
		WorkingDirectory: "/my/pwd",
		Args:             []string{"--foo", "bar"},
		Env:              []string{"KEY=val"},
		Kind:             "resource",
		LoaderAddress:    "127.0.0.1:9999",
	})
	require.NoError(t, err)
	require.NotNil(t, stdout)
	require.NotNil(t, stderr)
	require.NotNil(t, cancel)

	exitCode, err := done.Result(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int32(0), exitCode)

	reqs := stream.sentRequests()
	require.Len(t, reqs, 1)
	start := reqs[0].GetStart()
	require.NotNil(t, start)
	assert.Equal(t, "/my/pwd", start.Pwd)
	assert.Equal(t, []string{"--foo", "bar"}, start.Args)
	assert.Equal(t, []string{"KEY=val"}, start.Env)
	assert.Equal(t, "resource", start.Kind)
	assert.Equal(t, "127.0.0.1:9999", start.LoaderTarget)
}

func TestRunPlugin2StreamsOutput(t *testing.T) {
	t.Parallel()

	stream := newMockRunPlugin2Stream(t.Context())
	mockClient := &MockLanguageRuntimeClient{
		RunPlugin2F: func(ctx context.Context) (pulumirpc.LanguageRuntime_RunPlugin2Client, error) {
			return stream, nil
		},
	}

	pCtx, err := NewContext(t.Context(), nil, nil, nil, nil, "", nil, false, nil, nil)
	require.NoError(t, err)
	host := &langhost{ctx: pCtx, runtime: "test", client: mockClient}

	stream.recvCh <- &pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Stdout{Stdout: []byte("hello out\n")},
	}
	stream.recvCh <- &pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Stderr{Stderr: []byte("hello err\n")},
	}
	stream.recvCh <- &pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: 42},
	}
	close(stream.recvCh)

	stdout, stderr, _, done, err := host.RunPlugin(pCtx.Request(), RunPluginInfo{})
	require.NoError(t, err)

	var outBytes, errBytes []byte
	var outErr, errErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		outBytes, outErr = io.ReadAll(stdout)
	}()
	go func() {
		defer wg.Done()
		errBytes, errErr = io.ReadAll(stderr)
	}()
	wg.Wait()

	require.NoError(t, outErr)
	require.Equal(t, "hello out\n", string(outBytes))
	require.NoError(t, errErr)
	require.Equal(t, "hello err\n", string(errBytes))

	exitCode, err := done.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, int32(42), exitCode)
}

func TestRunPlugin2SoftCancel(t *testing.T) {
	t.Parallel()

	stream := newMockRunPlugin2Stream(t.Context())
	mockClient := &MockLanguageRuntimeClient{
		RunPlugin2F: func(ctx context.Context) (pulumirpc.LanguageRuntime_RunPlugin2Client, error) {
			return stream, nil
		},
	}

	pCtx, err := NewContext(t.Context(), nil, nil, nil, nil, "", nil, false, nil, nil)
	require.NoError(t, err)
	host := &langhost{ctx: pCtx, runtime: "test", client: mockClient}

	_, _, cancel, _, err := host.RunPlugin(pCtx.Request(), RunPluginInfo{})
	require.NoError(t, err)

	cancel(false)
	cancel(false) // Calling soft cancel again should be a no-op

	reqs := stream.sentRequests()
	require.Len(t, reqs, 2, "expected start + one soft cancel")
	require.NotNil(t, reqs[0].GetStart())
	require.NotNil(t, reqs[1].GetCancel())
	require.False(t, reqs[1].GetCancel().Force)

	close(stream.recvCh)
}

func TestRunPlugin2HardCancel(t *testing.T) {
	t.Parallel()

	stream := newMockRunPlugin2Stream(t.Context())
	mockClient := &MockLanguageRuntimeClient{
		RunPlugin2F: func(ctx context.Context) (pulumirpc.LanguageRuntime_RunPlugin2Client, error) {
			return stream, nil
		},
	}

	pCtx, err := NewContext(t.Context(), nil, nil, nil, nil, "", nil, false, nil, nil)
	require.NoError(t, err)
	host := &langhost{ctx: pCtx, runtime: "test", client: mockClient}

	_, _, cancel, _, err := host.RunPlugin(pCtx.Request(), RunPluginInfo{})
	require.NoError(t, err)

	cancel(false)
	cancel(true)

	reqs := stream.sentRequests()
	require.Len(t, reqs, 3, "expected start + soft + hard")
	require.NotNil(t, reqs[0].GetStart())
	require.False(t, reqs[1].GetCancel().Force)
	require.True(t, reqs[2].GetCancel().Force)

	close(stream.recvCh)
}

func TestRunPlugin2FallsBackToRunPlugin(t *testing.T) {
	t.Parallel()

	runPluginCalled := false
	mockClient := &MockLanguageRuntimeClient{
		RunPlugin2F: nil, // default returns Unimplemented
		RunPluginF: func(
			ctx context.Context, info *pulumirpc.RunPluginRequest,
		) (pulumirpc.LanguageRuntime_RunPluginClient, error) {
			runPluginCalled = true
			require.Equal(t, "/my/pwd", info.Pwd)
			return nil, assert.AnError
		},
	}

	pCtx, err := NewContext(t.Context(), nil, nil, nil, nil, "", nil, false, nil, nil)
	require.NoError(t, err)
	host := &langhost{ctx: pCtx, runtime: "test", client: mockClient}

	_, _, _, _, err = host.RunPlugin(pCtx.Request(), RunPluginInfo{
		WorkingDirectory: "/my/pwd",
	})
	require.ErrorIs(t, err, assert.AnError)
	require.True(t, runPluginCalled)
}
