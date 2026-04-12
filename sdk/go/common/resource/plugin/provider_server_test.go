// Copyright 2023, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Validate that Configure can read inputs from variables instead of args.
func TestProviderServer_Configure_variables(t *testing.T) {
	t.Parallel()

	provider := stubProvider{
		ConfigureFunc: func(pm resource.PropertyMap) error {
			assert.Equal(t, map[string]any{
				"foo": "bar",
				"baz": 42.0,
				"qux": map[string]any{
					"a": "str",
					"b": true,
				},
			}, pm.Mappable())
			return nil
		},
	}
	srv := NewProviderServer(&provider)

	ctx := t.Context()
	_, err := srv.Configure(ctx, &pulumirpc.ConfigureRequest{
		Variables: map[string]string{
			"ns:foo": `"bar"`,
			"ns:baz": "42",
			"ns:qux": `{"a": "str", "b": true}`,
		},
	})
	require.NoError(t, err)
}

// stubProvider is a Provider implementation
// with support for stubbing out specific methods.
type stubProvider struct {
	Provider

	CreateFunc func(
		urn resource.URN,
		inputs resource.PropertyMap,
	) (CreateResponse, error)

	ReadFunc func(
		urn resource.URN, id resource.ID,
		inputs, state resource.PropertyMap,
	) (ReadResult, resource.Status, error)

	ConfigureFunc func(resource.PropertyMap) error
}

func (p *stubProvider) Configure(ctx context.Context, req ConfigureRequest) (ConfigureResponse, error) {
	if p.ConfigureFunc != nil {
		err := p.ConfigureFunc(req.Inputs)
		return ConfigureResponse{}, err
	}
	return p.Provider.Configure(ctx, req)
}

func (p *stubProvider) Create(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	if p.CreateFunc != nil {
		return p.CreateFunc(req.URN, req.Properties)
	}

	return p.Provider.Create(ctx, req)
}

func (p *stubProvider) Read(ctx context.Context, req ReadRequest) (ReadResponse, error) {
	if p.ReadFunc != nil {
		props, status, err := p.ReadFunc(req.URN, req.ID, req.Inputs, req.State)
		return ReadResponse{
			ReadResult: props,
			Status:     status,
		}, err
	}
	return p.Provider.Read(ctx, req)
}

func TestProviderServer_Create_mapsAlreadyExists(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	provider := stubProvider{
		CreateFunc: func(
			urn resource.URN,
			inputs resource.PropertyMap,
		) (CreateResponse, error) {
			return CreateResponse{}, &AlreadyExistsError{Cause: "conflicting remote resource"}
		},
	}

	srv := NewProviderServer(&provider)
	_, err := srv.Create(ctx, &pulumirpc.CreateRequest{
		Urn: "urn:pulumi:dev::project::pkg:index:Thing::thing",
	})
	require.Error(t, err)

	statusErr, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, statusErr.Code())
	assert.Equal(t, "conflicting remote resource", statusErr.Message())
}

// When importing random passwords, the secret passed as "ID" should not leak in plain text into the final ID.
func TestProviderServer_Read_respects_ID(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	provider := stubProvider{
		ReadFunc: func(
			urn resource.URN, id resource.ID,
			inputs, state resource.PropertyMap,
		) (ReadResult, resource.Status, error) {
			return ReadResult{
				ID: resource.ID("none"),
				Outputs: resource.NewPropertyMapFromMap(map[string]any{
					"result": resource.NewProperty(&resource.Secret{
						Element: resource.NewProperty(string(id)),
					}),
				}),
			}, resource.StatusOK, nil
		},
	}
	secret := "supersecretpassword"
	srv := NewProviderServer(&provider)
	resp, err := srv.Read(ctx, &pulumirpc.ReadRequest{
		Urn: "urn:pulumi:v2::re::random:index/randomPassword:RandomPassword::newPassword",
		Id:  secret,
	})
	require.NoError(t, err)
	require.NotEqual(t, secret, resp.Id)
}
