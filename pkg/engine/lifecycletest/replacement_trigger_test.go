// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestReplacementTrigger(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	value := resource.NewPropertyValue("first")
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:             resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"}),
			ReplacementTrigger: value,
		})

		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")

	snap, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			for _, e := range events {
				if e.Type == ResourcePreEvent && e.Payload().(ResourcePreEventPayload).Metadata.URN.Name() == "resA" {
					assert.Equal(t, deploy.OpSame, e.Payload().(ResourcePreEventPayload).Metadata.Op)
				}
			}
			return nil
		},
		"1",
	)
	require.NoError(t, err)

	assert.Equal(t, 2, len(snap.Resources))
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")

	value = resource.NewPropertyValue("second")

	snap, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			operations := []display.StepOp{}

			for _, e := range events {
				if e.Type == ResourcePreEvent && e.Payload().(ResourcePreEventPayload).Metadata.URN.Name() == "resA" {
					operations = append(operations, e.Payload().(ResourcePreEventPayload).Metadata.Op)
				}
			}

			assert.Contains(t, operations, deploy.OpReplace)
			return nil
		},
		"2",
	)
	require.NoError(t, err)

	assert.Equal(t, 2, len(snap.Resources))
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestReplacementTriggerWithSecret(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	value := resource.NewPropertyValue("first")
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:             resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"}),
			ReplacementTrigger: value,
		})

		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")

	// Making this value secret should not trigger a replace, as the underlying value is still the same.
	value = resource.MakeSecret(value)

	snap, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			for _, e := range events {
				if e.Type == ResourcePreEvent && e.Payload().(ResourcePreEventPayload).Metadata.URN.Name() == "resA" {
					assert.Equal(t, deploy.OpSame, e.Payload().(ResourcePreEventPayload).Metadata.Op)
				}
			}
			return nil
		},
		"1",
	)
	require.NoError(t, err)

	assert.Equal(t, 2, len(snap.Resources))
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestReplacementTriggerWithDeepSecret(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	value := resource.NewPropertyValue([]resource.PropertyValue{resource.NewPropertyValue("first")})
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:             resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"}),
			ReplacementTrigger: value,
		})

		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")

	// Making the inner value secret should not trigger a replace, as the underlying value is still the same.
	value = resource.NewPropertyValue([]resource.PropertyValue{resource.MakeSecret(resource.NewPropertyValue("first"))})

	snap, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			for _, e := range events {
				if e.Type == ResourcePreEvent && e.Payload().(ResourcePreEventPayload).Metadata.URN.Name() == "resA" {
					assert.Equal(t, deploy.OpSame, e.Payload().(ResourcePreEventPayload).Metadata.Op)
				}
			}
			return nil
		},
		"1",
	)
	require.NoError(t, err)

	assert.Equal(t, 2, len(snap.Resources))
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestReplacementTriggerWithOutput(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	// A known output should not trigger a replace.
	value := resource.NewProperty(resource.Output{
		Element:      resource.NewPropertyValue("first"),
		Known:        true,
		Secret:       false,
		Dependencies: []resource.URN{},
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:             resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"}),
			ReplacementTrigger: value,
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")

	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			for _, e := range events {
				if e.Type == ResourcePreEvent && e.Payload().(ResourcePreEventPayload).Metadata.URN.Name() == "resA" {
					assert.Equal(t, deploy.OpSame, e.Payload().(ResourcePreEventPayload).Metadata.Op)
				}
			}
			return nil
		}, "1")

	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")

	// Making this value an unknown output should always trigger a replace.
	value = resource.NewProperty(resource.Output{
		Element:      resource.NewPropertyValue("first"),
		Known:        false,
		Secret:       false,
		Dependencies: []resource.URN{},
	})

	snap, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			operations := []display.StepOp{}

			for _, e := range events {
				if e.Type == ResourcePreEvent && e.Payload().(ResourcePreEventPayload).Metadata.URN.Name() == "resA" {
					operations = append(operations, e.Payload().(ResourcePreEventPayload).Metadata.Op)
				}
			}

			assert.Contains(t, operations, deploy.OpReplace)
			return nil
		}, "2")
	require.NoError(t, err)

	assert.Equal(t, 2, len(snap.Resources))
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestReplacementTriggerWithComputed(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	value := resource.MakeComputed(resource.NewPropertyValue("first"))
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:             resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"}),
			ReplacementTrigger: value,
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")

	// Unknown values during preview runs should trigger a replace.
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			operations := []display.StepOp{}

			for _, e := range events {
				if e.Type == ResourcePreEvent && e.Payload().(ResourcePreEventPayload).Metadata.URN.Name() == "resA" {
					operations = append(operations, e.Payload().(ResourcePreEventPayload).Metadata.Op)
				}
			}

			assert.Contains(t, operations, deploy.OpReplace)
			return nil
		}, "1")

	require.NoError(t, err)

	value = resource.MakeComputed(resource.NewPropertyValue("first"))

	// Unknown values during non-preview runs should trigger an error.
	snap, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			for _, e := range events {
				if e.Type == DiagEvent {
					diag := e.Payload().(DiagEventPayload).Message

					if strings.Contains(diag, "replacement trigger contains unknowns for urn:pulumi:test::test::pkgA:m:typA::resA") {
						return nil
					}
				}
			}

			assert.Fail(t, "expected matching diag event")
			return nil
		}, "2")
	require.NoError(t, err)

	assert.Equal(t, 2, len(snap.Resources))
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}
