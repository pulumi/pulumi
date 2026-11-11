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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// The CheckConfig RPC is exposed in Go as two methods, CheckConfig and CheckConfig2. These servers implement one,
// both or neither of them.

type checkConfigOldServer struct {
	pulumirpc.UnimplementedResourceProviderServer
}

func (checkConfigOldServer) CheckConfig(
	_ context.Context, req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{
		Inputs:   req.GetNews(),
		Failures: []*pulumirpc.CheckFailure{{Property: "region", Reason: "old"}},
	}, nil
}

type checkConfigNewServer struct {
	pulumirpc.UnimplementedResourceProviderServer
}

func (checkConfigNewServer) CheckConfig2(
	_ context.Context, req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckConfigResponse, error) {
	return &pulumirpc.CheckConfigResponse{
		Inputs:   req.GetNews(),
		Failures: []*pulumirpc.CheckConfigFailure{{Property: "region", Reason: "new"}},
	}, nil
}

type checkConfigBothServer struct {
	checkConfigNewServer
}

func (checkConfigBothServer) CheckConfig(context.Context, *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return nil, status.Error(codes.Internal, "CheckConfig should not be called when CheckConfig2 is implemented")
}

type checkConfigFailingServer struct {
	checkConfigBothServer
}

func (checkConfigFailingServer) CheckConfig2(
	context.Context, *pulumirpc.CheckRequest,
) (*pulumirpc.CheckConfigResponse, error) {
	return nil, status.Error(codes.Unknown, "credentials expired")
}

func serveCheckConfig(t *testing.T, srv pulumirpc.ResourceProviderServer) pulumirpc.ResourceProviderClient {
	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pulumirpc.RegisterResourceProviderServer(server, srv)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pulumirpc.NewResourceProviderClient(conn)
}

func checkConfigTestRPCRequest(t *testing.T) *pulumirpc.CheckRequest {
	news, err := structpb.NewStruct(map[string]any{"region": "us-west-2"})
	require.NoError(t, err)
	return &pulumirpc.CheckRequest{
		Urn:  "urn:pulumi:org/proj/dev::foo::pulumi:providers:pkg::default",
		News: news,
	}
}

// Whichever method the server implements, and whichever method the client calls, the answer is the same.
func TestCheckConfig_bothGoMethodsServeOneRPC(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		server pulumirpc.ResourceProviderServer
		reason string
	}{
		{"server implements CheckConfig", checkConfigOldServer{}, "old"},
		{"server implements CheckConfig2", checkConfigNewServer{}, "new"},
		{"server implements both", checkConfigBothServer{}, "new"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := serveCheckConfig(t, tt.server)
			req := checkConfigTestRPCRequest(t)

			resp, err := client.CheckConfig(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, "us-west-2", resp.GetInputs().GetFields()["region"].GetStringValue())
			require.Len(t, resp.GetFailures(), 1)
			assert.Equal(t, "region", resp.GetFailures()[0].GetProperty())
			assert.Equal(t, tt.reason, resp.GetFailures()[0].GetReason())

			resp2, err := client.CheckConfig2(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, "us-west-2", resp2.GetInputs().GetFields()["region"].GetStringValue())
			require.Len(t, resp2.GetFailures(), 1)
			assert.Equal(t, "region", resp2.GetFailures()[0].GetProperty())
			assert.Equal(t, tt.reason, resp2.GetFailures()[0].GetReason())
		})
	}
}

func TestCheckConfig_neitherGoMethodImplemented(t *testing.T) {
	t.Parallel()

	client := serveCheckConfig(t, pulumirpc.UnimplementedResourceProviderServer{})
	req := checkConfigTestRPCRequest(t)

	_, err := client.CheckConfig(t.Context(), req)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
	assert.ErrorContains(t, err, "method CheckConfig not implemented")

	_, err = client.CheckConfig2(t.Context(), req)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
	assert.ErrorContains(t, err, "method CheckConfig not implemented")
}

func TestCheckConfig_doesNotFallBackOnOtherErrors(t *testing.T) {
	t.Parallel()

	client := serveCheckConfig(t, checkConfigFailingServer{})

	_, err := client.CheckConfig2(t.Context(), checkConfigTestRPCRequest(t))
	assert.Equal(t, codes.Unknown, status.Code(err))
	assert.ErrorContains(t, err, "credentials expired")
}

func TestProvider_CheckConfig(t *testing.T) {
	t.Parallel()

	req := CheckConfigRequest{
		URN:  resource.NewURN("org/proj/dev", "foo", "", "pulumi:providers:pkg", "default"),
		News: property.NewMap(map[string]property.Value{"region": property.New("us-west-2")}),
	}

	for name, server := range map[string]pulumirpc.ResourceProviderServer{
		"old": checkConfigOldServer{},
		"new": checkConfigNewServer{},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := NewProviderWithClient(newTestContext(t), serveCheckConfig(t, server), false /* disablePreview */)
			resp, err := p.CheckConfig(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, property.New("us-west-2"), resp.Properties.Get("region"))
			assert.Equal(t, []CheckFailure{{Property: "region", Reason: name}}, resp.Failures)
		})
	}

	t.Run("unimplemented", func(t *testing.T) {
		t.Parallel()

		client := serveCheckConfig(t, pulumirpc.UnimplementedResourceProviderServer{})
		p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
		resp, err := p.CheckConfig(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, req.News, resp.Properties)
		assert.Empty(t, resp.Failures)
	})
}

type checkConfigProvider struct {
	UnimplementedProvider
}

func (p *checkConfigProvider) CheckConfig(_ context.Context, req CheckConfigRequest) (CheckConfigResponse, error) {
	return CheckConfigResponse{
		Properties: req.News,
		Failures:   []CheckFailure{{Property: "region", Reason: string(req.Type) + "/" + req.Name}},
	}, nil
}

func TestProviderServer_CheckConfig2_matchesCheckConfig(t *testing.T) {
	t.Parallel()

	srv := NewProviderServer(&checkConfigProvider{})

	resp, err := srv.CheckConfig(t.Context(), checkConfigTestRPCRequest(t))
	require.NoError(t, err)
	resp2, err := srv.CheckConfig2(t.Context(), checkConfigTestRPCRequest(t))
	require.NoError(t, err)

	assert.Equal(t, "us-west-2", resp2.GetInputs().GetFields()["region"].GetStringValue())
	assert.Equal(t, resp.GetInputs().AsMap(), resp2.GetInputs().AsMap())
	require.Len(t, resp.GetFailures(), 1)
	require.Len(t, resp2.GetFailures(), 1)
	assert.Equal(t, "region", resp2.GetFailures()[0].GetProperty())
	assert.Equal(t, "pulumi:providers:pkg/default", resp2.GetFailures()[0].GetReason())
	assert.Equal(t, resp.GetFailures()[0].GetProperty(), resp2.GetFailures()[0].GetProperty())
	assert.Equal(t, resp.GetFailures()[0].GetReason(), resp2.GetFailures()[0].GetReason())

	req := checkConfigTestRPCRequest(t)
	req.Name = "other"
	_, err = srv.CheckConfig(t.Context(), req)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
