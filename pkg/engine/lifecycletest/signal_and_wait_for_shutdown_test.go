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
	"errors"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:staticcheck,revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalAndWaitForShutdown(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	callSignalAndWaitForShutdown := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)
		if callSignalAndWaitForShutdown {
			err = monitor.SignalAndWaitForShutdown(context.Background())
			require.NoError(t, err)
		}
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")

	// Operation runs to completion even if we don't call WaitForShutdown
	callSignalAndWaitForShutdown = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestSignalAndWaitForShutdownError(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// Fail the creation of the resource
				CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{}, errors.New("oh no")
				},
			}, nil
		}),
	}

	callSignalAndWaitForShutdown := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		if callSignalAndWaitForShutdown {
			err = monitor.SignalAndWaitForShutdown(context.Background())
			// The `RegisterResource` call above resulted in an error, causing
			// the monitor to shutdown. However the call to `WaitForShutdown`
			// here can race with the shutdown. If we get the call in before the
			// monitor starts shutting down, we return successfully, otherwise
			// we get a connection refused.
			if err != nil {
				require.ErrorContains(t, err, "connection refused", "The resource monitor has already shut down")
			}
		}
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	project := p.GetProject()

	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "oh no")

	// Operation runs to completion with the expected error even if we don't call WaitForShutdown
	callSignalAndWaitForShutdown = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "oh no")
}

func TestSignalAndWaitForShutdownContinueOnError(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// Fail the creation of the resource
				CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{}, errors.New("oh no")
				},
			}, nil
		}),
	}

	callSignalAndWaitForShutdown := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.ErrorContains(t, err, "resource registration failed")
		if callSignalAndWaitForShutdown {
			err = monitor.SignalAndWaitForShutdown(context.Background())
			// Even though RegisterResource returned an error, we continue
			// running and expect WaitForShutdown to return successfully because
			// we are running with `ContinueOnError: true`.
			require.NoError(t, err)
		}
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, UpdateOptions: UpdateOptions{
			ContinueOnError: true,
		}},
	}
	project := p.GetProject()

	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "oh no")

	// Operation runs to completion with the expected error even if we don't call WaitForShutdown
	callSignalAndWaitForShutdown = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "oh no")
}

func TestSignalAndWaitForShutdownErrorAfterWait(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	callSignalAndWaitForShutdown := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)
		if callSignalAndWaitForShutdown {
			err = monitor.SignalAndWaitForShutdown(context.Background())
			require.NoError(t, err)
			return errors.New("error in program after signal")
		}
		return errors.New("error in program without signal")
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	project := p.GetProject()

	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.ErrorContains(t, err, "error in program after signal")

	// Operation runs to completion with the expected error even if we don't call WaitForShutdown
	callSignalAndWaitForShutdown = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "error in program without signal")
}
