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
	"strings"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

// hasDiagWarning returns true if any event is a diagnostic warning containing the given substring.
func hasDiagWarning(evts []Event, substr string) bool {
	for _, evt := range evts {
		if evt.Type != DiagEvent {
			continue
		}
		d := evt.Payload().(DiagEventPayload)
		if d.Severity == diag.Warning && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

// TestProviderWarnings_Create verifies that warnings returned by a provider's Create method
// are forwarded to the diagnostic event stream.
func TestProviderWarnings_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id"),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
						Warnings:   []string{"create warning from provider"},
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
		require.NoError(t, err)
		require.True(t, hasDiagWarning(evts, "create warning from provider"),
			"expected warning from Create to appear in diagnostic events")
		return nil
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate, "0")
	require.NoError(t, err)
}

// TestProviderWarnings_Check verifies that warnings returned by a provider's Check method
// are forwarded to the diagnostic event stream.
func TestProviderWarnings_Check(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{
						Properties: req.News,
						Warnings:   []string{"check warning from provider"},
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id"),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
		require.NoError(t, err)
		require.True(t, hasDiagWarning(evts, "check warning from provider"),
			"expected warning from Check to appear in diagnostic events")
		return nil
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate, "0")
	require.NoError(t, err)
}

// TestProviderWarnings_Update verifies that warnings returned by a provider's Update method
// are forwarded to the diagnostic event stream. Uses a 2-step approach:
// step 0 creates an initial resource, step 1 updates it and expects the warning.
func TestProviderWarnings_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if req.OldOutputs["v"] != req.NewInputs["v"] {
						return plugin.DiffResult{Changes: plugin.DiffSome}, nil
					}
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
						Warnings:   []string{"update warning from provider"},
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "initial"})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Step 0: create the resource.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Change input to trigger an update.
	inputs["v"] = resource.NewProperty("updated")

	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
		require.NoError(t, err)
		require.True(t, hasDiagWarning(evts, "update warning from provider"),
			"expected warning from Update to appear in diagnostic events")
		return nil
	}

	// Step 1: update the resource, expect warnings.
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	require.NoError(t, err)
}

// TestProviderWarnings_Diff verifies that warnings returned by a provider's Diff method
// are forwarded to the diagnostic event stream.
func TestProviderWarnings_Diff(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("created-id"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if req.OldOutputs["v"] != req.NewInputs["v"] {
						return plugin.DiffResult{
							Changes:  plugin.DiffSome,
							Warnings: []string{"diff warning from provider"},
						}, nil
					}
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "initial"})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Step 0: create the resource.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Change input to trigger a diff.
	inputs["v"] = resource.NewProperty("updated")

	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
		require.NoError(t, err)
		require.True(t, hasDiagWarning(evts, "diff warning from provider"),
			"expected warning from Diff to appear in diagnostic events")
		return nil
	}

	// Step 1: update the resource, expect diff warning.
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	require.NoError(t, err)
}
