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

package lifecycletest

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An await is not a failure: error hooks must not run and Create must be called exactly once per update, even
// when an OnError hook would ask for a retry.
func TestAwaitErrorCreateSkipsErrorHooks(t *testing.T) {
	t.Parallel()

	var createCalls, hookCalls atomic.Int32

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						return plugin.CreateResponse{Status: resource.StatusOK}, nil
					}
					createCalls.Add(1)
					return plugin.CreateResponse{Status: resource.StatusOK}, &plugin.AwaitError{}
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hook, err := deploytest.NewErrorHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string,
				_ tokens.Type, _, _ *pulumirpc.ResourceOptions, _, _, _ resource.PropertyMap,
				_ string, _ []string,
			) (bool, error) {
				hookCalls.Add(1)
				return true, nil // always ask for a retry
			})
		require.NoError(t, err)

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			ResourceHookBindings:    deploytest.ResourceHookBindings{OnError: []*deploytest.ResourceHook{hook}},
		})
		// Today the registration fails once the hook retry limit is reached, so don't require success here;
		// the call counts below carry the assertion.
		if err == nil {
			assert.True(t, resp.Unknown)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	_, err := lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.Error(t, err, "an await still surfaces as an overall error")

	t.Logf("observed Create calls=%d, error hook calls=%d", createCalls.Load(), hookCalls.Load())
	assert.Equal(t, int32(0), hookCalls.Load(), "error hooks must not run for an await")
	assert.Equal(t, int32(1), createCalls.Load(), "Create must be called exactly once for an await")
}
