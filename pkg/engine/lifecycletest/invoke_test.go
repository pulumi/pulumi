// Copyright 2025-2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, _, err := monitor.Invoke("pkgA:index:myFunc", nil, "", "", "")
		require.NoError(t, err)
		assert.Equal(t, resource.NewProperty("invoked"), resp["result"])
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)

	expectPreview = false
	_, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
