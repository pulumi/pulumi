// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunQuery_nocreate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Outputs: resource.PropertyMap{},
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.ErrorContains(t, err, "Query mode does not support creating, updating, or deleting resources")

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.ErrorContains(t, err, "Query mode does not support creating, updating, or deleting resources")

		_, _, err = monitor.ReadResource("pkgA:m:typA", "resA", "read-id", "", nil, "", "", "", "")
		assert.ErrorContains(t, err, "Query mode does not support reading resources")
		return nil
	})

	plugCtx, err := plugin.NewContext(
		diagtest.LogSink(t), diagtest.LogSink(t),
		deploytest.NewPluginHostF(nil, nil, programF, loaders...)(),
		nil, "", nil, false, nil)
	assert.NoError(t, err)

	src, err := deploy.NewQuerySource(context.Background(), plugCtx, &deploytest.BackendClient{}, &deploy.EvalRunInfo{
		ProjectRoot: "/",
		Pwd:         "/",
		Program:     ".",
		Proj: &workspace.Project{
			Name: "query-program",
		},
	}, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, src.Wait())
}

func TestRunQuery_call_invoke(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					name := req.Args["name"]
					ret := "unexpected"
					if name.IsString() {
						ret = "Hello, " + name.StringValue() + "!"
					}

					return plugin.InvokeResponse{
						Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
							"message": ret,
						}),
					}, nil
				},
				CallF: func(
					_ context.Context,
					req plugin.CallRequest,
					_ *deploytest.ResourceMonitor,
				) (plugin.CallResponse, error) {
					ret := "unexpected"
					if req.Args["name"].IsString() {
						ret = "Hello, " + req.Args["name"].StringValue() + "!"
					}

					return plugin.CallResponse{
						Return: resource.NewPropertyMapFromMap(map[string]interface{}{
							"message": ret,
						}),
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		outs, _, _, err := monitor.Call("pkgA:m:typA/methodA", resource.PropertyMap{
			"name": resource.NewStringProperty("bar"),
		}, nil, "", "", "")
		assert.NoError(t, err)
		assert.Equal(t, (resource.PropertyMap{
			"message": resource.NewStringProperty("Hello, bar!"),
		}), outs, "outs was %v", outs)

		outs, _, err = monitor.Invoke("pkgA:m:invokeA", resource.PropertyMap{
			"name": resource.NewStringProperty("bar"),
		}, "", "", "")
		assert.NoError(t, err)
		assert.Equal(t, (resource.PropertyMap{
			"message": resource.NewStringProperty("Hello, bar!"),
		}), outs, "outs was %v", outs)

		return nil
	})

	plugCtx, err := plugin.NewContext(
		diagtest.LogSink(t), diagtest.LogSink(t),
		deploytest.NewPluginHostF(nil, nil, programF, loaders...)(),
		nil, "", nil, false, nil)
	require.NoError(t, err)

	src, err := deploy.NewQuerySource(context.Background(), plugCtx, &deploytest.BackendClient{}, &deploy.EvalRunInfo{
		ProjectRoot: "/",
		Pwd:         "/",
		Program:     ".",
		Proj: &workspace.Project{
			Name: "query-program",
		},
	}, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, src.Wait())
}
