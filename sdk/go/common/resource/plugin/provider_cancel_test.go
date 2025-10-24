// Copyright 2016-2025, Pulumi Corporation.
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestProviderSignalCancellation_UnimplementedIsHandledGracefully(t *testing.T) {
	t.Parallel()

	var cancelCalled bool
	var receivedRequest *pulumirpc.CancelRequest

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CancelF: func(req *pulumirpc.CancelRequest) (*emptypb.Empty, error) {
			cancelCalled = true
			receivedRequest = req
			return nil, status.Error(codes.Unimplemented, "Cancel not implemented")
		},
	}

	p := NewProviderWithClient(newTestContext(t), "test-provider", client, false)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err)

	err = p.SignalCancellation(context.Background())

	require.NoError(t, err, "Unimplemented error should be handled gracefully")
	assert.True(t, cancelCalled, "Cancel RPC should have been called")
	require.NotNil(t, receivedRequest, "CancelRequest should have been sent")
}

func TestProviderSignalCancellation_SendsCancelRequest(t *testing.T) {
	t.Parallel()

	var receivedRequest *pulumirpc.CancelRequest

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CancelF: func(req *pulumirpc.CancelRequest) (*emptypb.Empty, error) {
			receivedRequest = req
			return &emptypb.Empty{}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), "test-provider", client, false)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err)

	err = p.SignalCancellation(context.Background())

	require.NoError(t, err)
	require.NotNil(t, receivedRequest, "should receive CancelRequest message")
}

func TestProviderSignalCancellation_PropagatesOtherErrors(t *testing.T) {
	t.Parallel()

	testErr := status.Error(codes.Internal, "internal provider error")

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CancelF: func(req *pulumirpc.CancelRequest) (*emptypb.Empty, error) {
			return nil, testErr
		},
	}

	p := NewProviderWithClient(newTestContext(t), "test-provider", client, false)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err)

	err = p.SignalCancellation(context.Background())

	assert.Error(t, err, "non-Unimplemented errors should be propagated")
	assert.Contains(t, err.Error(), "internal provider error")
}

func TestProviderSignalCancellation_RespectsContextCancellation(t *testing.T) {
	t.Parallel()

	cancelReceived := make(chan struct{})

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CancelF: func(req *pulumirpc.CancelRequest) (*emptypb.Empty, error) {
			close(cancelReceived)
			time.Sleep(5 * time.Second)
			return &emptypb.Empty{}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), "test-provider", client, false)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = p.SignalCancellation(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err, "should error on context timeout")
	assert.Less(t, elapsed, 2*time.Second, "should respect context timeout, not wait 5s")
	select {
	case <-cancelReceived:
	case <-time.After(200 * time.Millisecond):
		t.Error("Cancel RPC should have been called")
	}
}

func TestProviderSignalCancellation_HandlesUnavailableError(t *testing.T) {
	t.Parallel()

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CancelF: func(req *pulumirpc.CancelRequest) (*emptypb.Empty, error) {
			return nil, status.Error(codes.Unavailable, "provider unavailable")
		},
	}

	p := NewProviderWithClient(newTestContext(t), "test-provider", client, false)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err)

	err = p.SignalCancellation(context.Background())

	assert.Error(t, err, "Unavailable error should be propagated")
	assert.Contains(t, err.Error(), "Unavailable")
}

func TestProviderSignalCancellation_EmptyCancelRequest(t *testing.T) {
	t.Parallel()

	var receivedRequest *pulumirpc.CancelRequest

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CancelF: func(req *pulumirpc.CancelRequest) (*emptypb.Empty, error) {
			receivedRequest = req
			return &emptypb.Empty{}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), "test-provider", client, false)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err)

	err = p.SignalCancellation(context.Background())

	require.NoError(t, err)
	require.NotNil(t, receivedRequest)
	assert.Empty(t, receivedRequest.String(), "CancelRequest should be empty for MVP")
}

func TestProviderSignalCancellation_MultipleCallsAllowed(t *testing.T) {
	t.Parallel()

	callCount := 0

	client := &stubClient{
		ConfigureF: func(req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
			return &pulumirpc.ConfigureResponse{}, nil
		},
		CancelF: func(req *pulumirpc.CancelRequest) (*emptypb.Empty, error) {
			callCount++
			return &emptypb.Empty{}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), "test-provider", client, false)

	_, err := p.Configure(context.Background(), ConfigureRequest{})
	require.NoError(t, err)

	err = p.SignalCancellation(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	err = p.SignalCancellation(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "second Cancel call should succeed")
}
