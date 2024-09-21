// Copyright 2023-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
)

// Validate that Configure can read inputs from variables instead of args.
func TestProviderServer_Configure_variables(t *testing.T) {
	t.Parallel()

	provider := stubProvider{
		ConfigureFunc: func(pm resource.PropertyMap) error {
			assert.Equal(t, map[string]interface{}{
				"foo": "bar",
				"baz": 42.0,
				"qux": map[string]interface{}{
					"a": "str",
					"b": true,
				},
			}, pm.Mappable())
			return nil
		},
	}
	srv := NewProviderServer(&provider)

	ctx := context.Background()
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

// When importing random passwords, the secret passed as "ID" should not leak in plain text into the final ID.
func TestProviderServer_Read_respects_ID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider := stubProvider{
		ReadFunc: func(
			urn resource.URN, id resource.ID,
			inputs, state resource.PropertyMap,
		) (ReadResult, resource.Status, error) {
			return ReadResult{
				ID: resource.ID("none"),
				Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"result": resource.NewSecretProperty(&resource.Secret{
						Element: resource.NewStringProperty(string(id)),
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
