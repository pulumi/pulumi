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
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestExtensionParameterizedProvider exercises the full extension-parameterization
// flow end-to-end against a fake provider:
//
//   - The program registers two extensions on the same base provider via
//     RegisterPackage and creates one resource per extension.
//   - The fake provider's Parameterize is called twice (cumulatively, on the same
//     plugin instance) — once per extension blob.
//   - The resulting snapshot persists both extension blobs in snap.Extensions and
//     records each resource's ExtensionRef.
//   - A second Update against the same snapshot — without the program re-supplying
//     the blobs — exercises load-time rehydration: the engine pulls blobs from
//     state and replays Parameterize on the fresh plugin instance before any
//     resource ops touch it.
func TestExtensionParameterizedProvider(t *testing.T) {
	t.Parallel()

	// Track parameterize calls across the test lifetime so we can assert on
	// what blobs reached the provider, and from which run.
	type paramCall struct {
		name, version string
		value         []byte
	}
	var paramLock sync.Mutex
	var paramCalls []paramCall

	recordParameterize := func(value *plugin.ParameterizeValue) {
		paramLock.Lock()
		defer paramLock.Unlock()
		paramCalls = append(paramCalls, paramCall{
			name:    value.Name,
			version: value.Version.String(),
			value:   append([]byte(nil), value.Value...),
		})
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ParameterizeF: func(
					_ context.Context, req plugin.ParameterizeRequest,
				) (plugin.ParameterizeResponse, error) {
					value := req.Parameters.(*plugin.ParameterizeValue)
					recordParameterize(value)
					return plugin.ParameterizeResponse{
						Name:    value.Name,
						Version: value.Version,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	// Run #1: program registers two extensions and one resource per extension.
	var refA, refB string
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		refA, err = monitor.RegisterExtensionPackage("pkgA", "1.0.0", &pulumirpc.Parameterization{
			Name:    "ext-a",
			Version: "1.0.0",
			Value:   []byte("blob-a"),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			PackageRef: refA,
		})
		require.NoError(t, err)

		refB, err = monitor.RegisterExtensionPackage("pkgA", "1.0.0", &pulumirpc.Parameterization{
			Name:    "ext-b",
			Version: "1.0.0",
			Value:   []byte("blob-b"),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			PackageRef: refB,
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "")
	require.NoError(t, err)
	require.NotNil(t, snap)

	// Two parameterize calls happened during run #1.
	require.Len(t, paramCalls, 2, "expected two Parameterize calls in run #1")
	assert.Equal(t, "ext-a", paramCalls[0].name)
	assert.Equal(t, []byte("blob-a"), paramCalls[0].value)
	assert.Equal(t, "ext-b", paramCalls[1].name)
	assert.Equal(t, []byte("blob-b"), paramCalls[1].value)

	// Refs are content-stable hashes; identical blob ⇒ identical ref.
	assert.NotEqual(t, refA, refB, "different extensions must produce different refs")

	// Snapshot persists both blobs and links each resource to its ref.
	require.Len(t, snap.Extensions, 2, "snapshot should carry both extension blobs")
	assert.Equal(t, []byte("blob-a"), snap.Extensions[apitype.ExtensionRef(refA)].Value)
	assert.Equal(t, []byte("blob-b"), snap.Extensions[apitype.ExtensionRef(refB)].Value)

	var resA, resB *resource.State
	for _, r := range snap.Resources {
		switch r.URN.Name() {
		case "resA":
			resA = r
		case "resB":
			resB = r
		}
	}
	require.NotNil(t, resA, "resA missing from snapshot")
	require.NotNil(t, resB, "resB missing from snapshot")
	assert.Equal(t, refA, resA.ExtensionRef)
	assert.Equal(t, refB, resB.ExtensionRef)

	// Run #2: rehydration test. Use a program that does NOT register the
	// extensions (no live RegisterPackage); the engine must pull blobs from
	// state and re-Parameterize the fresh plugin before any resource op.
	paramLock.Lock()
	paramCalls = nil
	paramLock.Unlock()

	noopProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		// Program does nothing — engine processes existing resources from state.
		return nil
	})
	hostF2 := deploytest.NewPluginHostF(nil, nil, noopProgramF, loaders...)
	p2 := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF2, SkipDisplayTests: true}}

	snap2, err := lt.TestOp(Refresh).
		RunStep(p2.GetProject(), p2.GetTarget(t, snap), p2.Options, false, p2.BackendClient, nil, "")
	require.NoError(t, err)
	require.NotNil(t, snap2)

	// Rehydration must have replayed both blobs onto the fresh plugin.
	paramLock.Lock()
	rehydrated := append([]paramCall(nil), paramCalls...)
	paramLock.Unlock()
	require.Len(t, rehydrated, 2, "rehydration must replay both extensions on the fresh plugin")
	gotNames := []string{rehydrated[0].name, rehydrated[1].name}
	assert.ElementsMatch(t, []string{"ext-a", "ext-b"}, gotNames)
}
