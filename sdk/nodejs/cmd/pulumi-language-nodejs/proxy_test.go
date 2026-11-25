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

package main

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type shutdownMonitor struct {
	pulumirpc.UnimplementedResourceMonitorServer
}

func (m *shutdownMonitor) RegisterResource(
	ctx context.Context, req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	if req.GetName() == "reported" {
		contract.IgnoreError(grpc.SetTrailer(ctx, metadata.Pairs("pulumi-error-already-reported", "true")))
	}
	return nil, status.Error(codes.Unknown, "resource monitor shut down")
}

func (m *shutdownMonitor) ReadResource(
	ctx context.Context, req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	_, err := m.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{Name: req.GetName()})
	return nil, err
}

func (m *shutdownMonitor) RegisterResourceOutputs(
	ctx context.Context, req *pulumirpc.RegisterResourceOutputsRequest,
) (*emptypb.Empty, error) {
	_, err := m.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{Name: req.GetUrn()})
	return nil, err
}

func TestMonitorProxyPreservesErrorTrailers(t *testing.T) {
	t.Parallel()

	connect := func(monitor pulumirpc.ResourceMonitorServer) pulumirpc.ResourceMonitorClient {
		listener := bufconn.Listen(1024 * 1024)
		server := grpc.NewServer()
		pulumirpc.RegisterResourceMonitorServer(server, monitor)
		t.Cleanup(server.Stop)
		go func() {
			contract.IgnoreError(server.Serve(listener))
		}()
		conn, err := grpc.NewClient("passthrough:///monitor",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return listener.DialContext(ctx)
			}))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		return pulumirpc.NewResourceMonitorClient(conn)
	}

	client := connect(&monitorProxy{target: connect(&shutdownMonitor{})})

	for method, call := range map[string]func(context.Context, string, grpc.CallOption) error{
		"RegisterResource": func(ctx context.Context, name string, opt grpc.CallOption) error {
			_, err := client.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{Name: name}, opt)
			return err
		},
		"ReadResource": func(ctx context.Context, name string, opt grpc.CallOption) error {
			_, err := client.ReadResource(ctx, &pulumirpc.ReadResourceRequest{Name: name}, opt)
			return err
		},
		"RegisterResourceOutputs": func(ctx context.Context, name string, opt grpc.CallOption) error {
			_, err := client.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{Urn: name}, opt)
			return err
		},
	} {
		for _, name := range []string{"reported", "unreported"} {
			t.Run(method+"/"+name, func(t *testing.T) {
				t.Parallel()
				var trailer metadata.MD
				err := call(t.Context(), name, grpc.Trailer(&trailer))
				require.Equal(t, codes.Unknown, status.Code(err))
				assert.Equal(t, "resource monitor shut down", status.Convert(err).Message())
				if name == "reported" {
					assert.Equal(t, []string{"true"}, trailer.Get("pulumi-error-already-reported"))
				} else {
					assert.Empty(t, trailer.Get("pulumi-error-already-reported"))
				}
			})
		}
	}
}
