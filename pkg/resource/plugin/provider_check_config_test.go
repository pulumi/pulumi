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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type checkConfigClient struct {
	pulumirpc.ResourceProviderClient

	checkConfig  func(*pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error)
	checkConfig2 func(*pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error)
}

func (c *checkConfigClient) CheckConfig(
	_ context.Context, req *pulumirpc.CheckRequest, _ ...grpc.CallOption,
) (*pulumirpc.CheckResponse, error) {
	return c.checkConfig(req)
}

func (c *checkConfigClient) CheckConfig2(
	_ context.Context, req *pulumirpc.CheckConfigRequest, _ ...grpc.CallOption,
) (*pulumirpc.CheckConfigResponse, error) {
	return c.checkConfig2(req)
}

func checkConfigTestRequest() CheckConfigRequest {
	return CheckConfigRequest{
		URN:  resource.NewURN("org/proj/dev", "foo", "", "pulumi:providers:pkg", "default"),
		News: property.NewMap(map[string]property.Value{"region": property.New("us-west-2")}),
	}
}

func TestProvider_CheckConfig_prefersCheckConfig2(t *testing.T) {
	t.Parallel()

	var got *pulumirpc.CheckConfigRequest
	client := &checkConfigClient{
		checkConfig: func(*pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
			t.Error("CheckConfig should not be called when CheckConfig2 is implemented")
			return nil, status.Error(codes.Internal, "unexpected")
		},
		checkConfig2: func(req *pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
			got = req
			return &pulumirpc.CheckConfigResponse{
				Inputs:   req.GetNews(),
				Failures: []*pulumirpc.CheckConfigFailure{{Property: "region", Reason: "bad region"}},
			}, nil
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	resp, err := p.CheckConfig(t.Context(), checkConfigTestRequest())
	require.NoError(t, err)

	require.NotNil(t, got)
	assert.Equal(t, "urn:pulumi:org/proj/dev::foo::pulumi:providers:pkg::default", got.GetUrn())
	assert.Equal(t, "default", got.GetName())
	assert.Equal(t, "pulumi:providers:pkg", got.GetType())
	assert.Equal(t, "us-west-2", got.GetNews().GetFields()["region"].GetStringValue())

	assert.Equal(t, property.New("us-west-2"), resp.Properties.Get("region"))
	assert.Equal(t, []CheckFailure{{Property: "region", Reason: "bad region"}}, resp.Failures)
}

func TestProvider_CheckConfig_fallsBackToCheckConfig(t *testing.T) {
	t.Parallel()

	var got *pulumirpc.CheckRequest
	client := &checkConfigClient{
		checkConfig: func(req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
			got = req
			return &pulumirpc.CheckResponse{
				Inputs:   req.GetNews(),
				Failures: []*pulumirpc.CheckFailure{{Property: "region", Reason: "bad region"}},
			}, nil
		},
		checkConfig2: func(*pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
			return nil, status.Error(codes.Unimplemented, "method CheckConfig2 not implemented")
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	resp, err := p.CheckConfig(t.Context(), checkConfigTestRequest())
	require.NoError(t, err)

	require.NotNil(t, got)
	assert.Equal(t, "urn:pulumi:org/proj/dev::foo::pulumi:providers:pkg::default", got.GetUrn())
	assert.Equal(t, "default", got.GetName())
	assert.Equal(t, "pulumi:providers:pkg", got.GetType())
	assert.Equal(t, "us-west-2", got.GetNews().GetFields()["region"].GetStringValue())

	assert.Equal(t, property.New("us-west-2"), resp.Properties.Get("region"))
	assert.Equal(t, []CheckFailure{{Property: "region", Reason: "bad region"}}, resp.Failures)
}

func TestProvider_CheckConfig_neitherImplemented(t *testing.T) {
	t.Parallel()

	client := &checkConfigClient{
		checkConfig: func(*pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
			return nil, status.Error(codes.Unimplemented, "method CheckConfig not implemented")
		},
		checkConfig2: func(*pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
			return nil, status.Error(codes.Unimplemented, "method CheckConfig2 not implemented")
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	req := checkConfigTestRequest()
	resp, err := p.CheckConfig(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, req.News, resp.Properties)
	assert.Empty(t, resp.Failures)
}

func TestProvider_CheckConfig_doesNotFallBackOnOtherErrors(t *testing.T) {
	t.Parallel()

	client := &checkConfigClient{
		checkConfig: func(*pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
			t.Error("CheckConfig should not be called when CheckConfig2 fails with a real error")
			return nil, status.Error(codes.Internal, "unexpected")
		},
		checkConfig2: func(*pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
			return nil, status.Error(codes.Unknown, "credentials expired")
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	_, err := p.CheckConfig(t.Context(), checkConfigTestRequest())
	assert.ErrorContains(t, err, "credentials expired")
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

	urn := "urn:pulumi:org/proj/dev::foo::pulumi:providers:pkg::default"
	news, err := structpb.NewStruct(map[string]any{"region": "us-west-2"})
	require.NoError(t, err)

	srv := NewProviderServer(&checkConfigProvider{})

	resp, err := srv.CheckConfig(t.Context(), &pulumirpc.CheckRequest{Urn: urn, News: news})
	require.NoError(t, err)
	resp2, err := srv.CheckConfig2(t.Context(), &pulumirpc.CheckConfigRequest{Urn: urn, News: news})
	require.NoError(t, err)

	assert.Equal(t, "us-west-2", resp2.GetInputs().GetFields()["region"].GetStringValue())
	assert.Equal(t, resp.GetInputs().AsMap(), resp2.GetInputs().AsMap())
	require.Len(t, resp2.GetFailures(), 1)
	require.Len(t, resp.GetFailures(), 1)
	assert.Equal(t, "region", resp2.GetFailures()[0].GetProperty())
	assert.Equal(t, "pulumi:providers:pkg/default", resp2.GetFailures()[0].GetReason())
	assert.Equal(t, resp.GetFailures()[0].GetReason(), resp2.GetFailures()[0].GetReason())

	_, err = srv.CheckConfig2(t.Context(), &pulumirpc.CheckConfigRequest{Urn: urn, Name: "other", News: news})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestProviderServer_CheckConfig2_prefersProviderCheckConfig2(t *testing.T) {
	t.Parallel()

	urn := "urn:pulumi:org/proj/dev::foo::pulumi:providers:pkg::default"
	srv := NewProviderServer(&MockProvider{
		CheckConfigF: func(context.Context, CheckConfigRequest) (CheckConfigResponse, error) {
			return CheckConfigResponse{Failures: []CheckFailure{{Property: "region", Reason: "from CheckConfig"}}}, nil
		},
		CheckConfig2F: func(context.Context, CheckConfigRequest) (CheckConfig2Response, error) {
			return CheckConfig2Response{
				Failures: []CheckConfigFailure{{Property: "region", Reason: "from CheckConfig2"}},
			}, nil
		},
	})

	resp, err := srv.CheckConfig(t.Context(), &pulumirpc.CheckRequest{Urn: urn})
	require.NoError(t, err)
	require.Len(t, resp.GetFailures(), 1)
	assert.Equal(t, "from CheckConfig", resp.GetFailures()[0].GetReason())

	resp2, err := srv.CheckConfig2(t.Context(), &pulumirpc.CheckConfigRequest{Urn: urn})
	require.NoError(t, err)
	require.Len(t, resp2.GetFailures(), 1)
	assert.Equal(t, "from CheckConfig2", resp2.GetFailures()[0].GetReason())
}

func TestCheckConfig2_fallsBackToCheckConfig(t *testing.T) {
	t.Parallel()

	// checkConfigProvider does not implement ProviderWithCheckConfig2.
	req := checkConfigTestRequest()
	resp, err := CheckConfig2(t.Context(), &checkConfigProvider{}, req)
	require.NoError(t, err)
	assert.Equal(t, req.News, resp.Properties)
	assert.Equal(t, []CheckConfigFailure{{Property: "region", Reason: "/"}}, resp.Failures)
}

func TestCheckConfig2_carriesRemediations(t *testing.T) {
	t.Parallel()

	failures := []CheckConfigFailure{{
		Reason:       "credentials expired",
		Remediations: []CommandRemediation{{Description: "Sign in", Command: []string{"cloud", "login"}}},
	}}
	srv := NewProviderServer(&MockProvider{
		CheckConfig2F: func(context.Context, CheckConfigRequest) (CheckConfig2Response, error) {
			return CheckConfig2Response{Failures: failures}, nil
		},
	})
	client := &checkConfigClient{
		checkConfig: func(*pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
			t.Error("CheckConfig should not be called when CheckConfig2 is implemented")
			return nil, status.Error(codes.Internal, "unexpected")
		},
		checkConfig2: func(req *pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error) {
			return srv.CheckConfig2(t.Context(), req)
		},
	}

	p := NewProviderWithClient(newTestContext(t), client, false /* disablePreview */)
	resp, err := CheckConfig2(t.Context(), p, checkConfigTestRequest())
	require.NoError(t, err)
	assert.Equal(t, failures, resp.Failures)

	// CheckConfig reports the same failure without the remediation.
	old, err := p.CheckConfig(t.Context(), checkConfigTestRequest())
	require.NoError(t, err)
	assert.Equal(t, []CheckFailure{{Reason: "credentials expired"}}, old.Failures)
}
