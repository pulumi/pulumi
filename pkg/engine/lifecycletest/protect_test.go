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

	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestMultipleProtectedDeletes tests that a preview and up can see multiple protect delete errors.
func TestMultipleProtectedDeletes(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	creating := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		assert.NoError(t, err)

		if creating {
			protect := true
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Protect: &protect,
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Protect: &protect,
			})
			assert.NoError(t, err)
		}

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial update which sets some stack outputs.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, resource.RootStackType, snap.Resources[0].Type)

	// Run a preview that will try and delete both resA and resB.
	creating = false
	validate := func(
		project workspace.Project, target deploy.Target, entries engine.JournalEntries,
		events []engine.Event, err error,
	) error {
		// Check that we saw events for both resA and resB
		var sawA, sawB bool
		for _, e := range events {
			if e.Type == DiagEvent {
				payload := e.Payload().(engine.DiagEventPayload)
				if payload.URN != "" {
					if payload.URN.Name() == "resA" {
						assert.Contains(t, payload.Message, "resA\" cannot be deleted")
						sawA = true
					}
					if payload.URN.Name() == "resB" {
						assert.Contains(t, payload.Message, "resB\" cannot be deleted")
						sawB = true
					}
				}
			}
		}

		assert.True(t, sawA, "did not see resA")
		assert.True(t, sawB, "did not see resB")

		return err
	}
	_, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, true, p.BackendClient, validate, "1")
	assert.Error(t, err)

	// Run an update that will try and delete both resA and resB.
	_, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "2")
	assert.Error(t, err)
}

// TestProtectInheritance tests that the protect option is inherited from parent resources but can be overridden.
func TestProtectInheritance(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		protect := true
		resp, err := monitor.RegisterResource("my_component", "parent", false, deploytest.ResourceOptions{
			Protect: &protect,
		})
		assert.NoError(t, err)

		// Inherit protect true from parent
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		assert.NoError(t, err)

		// Override protect true from parent
		protectB := false
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Parent:  resp.URN,
			Protect: &protectB,
		})
		assert.NoError(t, err)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the update
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 4)
	// Assert that parent and resA are protected and resB is not
	assert.Equal(t, "parent", snap.Resources[0].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, "resB", snap.Resources[3].URN.Name())
	assert.True(t, snap.Resources[0].Protect)
	assert.True(t, snap.Resources[2].Protect)
	assert.False(t, snap.Resources[3].Protect)
	// Assert that resA and resB have the correct parent
	assert.Equal(t, snap.Resources[0].URN, snap.Resources[2].Parent)
	assert.Equal(t, snap.Resources[0].URN, snap.Resources[3].Parent)
}
