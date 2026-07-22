// Copyright 2025, Pulumi Corporation.
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

package lifecycletest

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestPreviewInvoke(t *testing.T) {
	t.Parallel()

	expectPreview := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					ctx context.Context, req plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					assert.True(t, req.InvokeWithPreview, "expected engine to advertise invoke_with_preview support")
					return &plugin.ProviderHandshakeResponse{}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, expectPreview, req.Preview)
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result": resource.NewProperty("invoked"),
						},
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, _, err := monitor.Invoke("pkgA:index:myFunc", nil, "", "", "")
		require.NoError(t, err)
		assert.Equal(t, resource.NewProperty("invoked"), resp["result"])
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)

	expectPreview = false
	_, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

// TestInvokeParentResolvesComponentProviders verifies that an invoke sent with a parent URN is
// served by the provider that parent's `providers` option names for the invoke's package, exactly
// as a resource registered under that parent would be.
func TestInvokeParentResolvesComponentProviders(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			instance := "default"
			return &deploytest.Provider{
				ConfigureF: func(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					if v, ok := req.Inputs["instance"]; ok && v.IsString() {
						instance = v.StringValue()
					}
					return plugin.ConfigureResponse{}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"instance": resource.NewProperty(instance),
						},
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		registerProvider := func(name string) string {
			resp, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), name, true,
				deploytest.ResourceOptions{
					Inputs: resource.PropertyMap{"instance": resource.NewProperty(name)},
				})
			require.NoError(t, err)
			ref, err := providers.NewReference(resp.URN, resp.ID)
			require.NoError(t, err)
			return ref.String()
		}
		explicitRef := registerProvider("explicit")
		otherRef := registerProvider("other")

		comp, err := monitor.RegisterResource("my:component:First", "comp", false, deploytest.ResourceOptions{
			Providers: map[string]string{"pkgA": explicitRef},
		})
		require.NoError(t, err)
		child, err := monitor.RegisterResource("my:component:Second", "child", false, deploytest.ResourceOptions{
			Parent: comp.URN,
		})
		require.NoError(t, err)
		custom, err := monitor.RegisterResource("pkgA:m:typA", "custom", true, deploytest.ResourceOptions{
			Parent: comp.URN,
		})
		require.NoError(t, err)

		servedBy := func(provider string, options ...deploytest.InvokeOptions) string {
			resp, _, err := monitor.Invoke("pkgA:index:echo", nil, provider, "", "", options...)
			require.NoError(t, err)
			require.True(t, resp["instance"].IsString())
			return resp["instance"].StringValue()
		}

		assert.Equal(t, "explicit", servedBy("", deploytest.InvokeOptions{Parent: comp.URN}),
			"an invoke parented to the component resolves the component's provider")
		assert.Equal(t, "explicit", servedBy("", deploytest.InvokeOptions{Parent: child.URN}),
			"the providers option is inherited through nested components")
		assert.Equal(t, "other", servedBy(otherRef, deploytest.InvokeOptions{Parent: comp.URN}),
			"an explicit provider wins over the parent's providers option")
		assert.Equal(t, "default", servedBy("", deploytest.InvokeOptions{Parent: custom.URN}),
			"a custom parent carries no providers option")
		assert.Equal(t, "default", servedBy(""),
			"an unparented invoke is served by the default provider")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

func TestSecretsInvoke(t *testing.T) {
	t.Parallel()

	expectPreview := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					ctx context.Context, req plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					return &plugin.ProviderHandshakeResponse{AcceptSecrets: false}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, expectPreview, req.Preview)
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result": resource.NewProperty("invoked"),
						},
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, _, err := monitor.Invoke("pkgA:index:myFunc", resource.PropertyMap{
			"secret": resource.MakeSecret(resource.NewProperty("my-secret")),
		}, "", "", "")
		require.NoError(t, err)
		assert.Equalf(t, resource.MakeSecret(resource.NewProperty("invoked")), resp["result"], "Returned: %#v", resp)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)

	expectPreview = false
	_, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
