// Copyright 2016-2018, Pulumi Corporation.
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

// nolint: goconst
package lifecycletest

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	combinations "github.com/mxschmitt/golang-combinations"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	. "github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
)

func SuccessfulSteps(entries JournalEntries) []deploy.Step {
	var steps []deploy.Step
	for _, entry := range entries {
		if entry.Kind == JournalEntrySuccess {
			steps = append(steps, entry.Step)
		}
	}
	return steps
}

type StepSummary struct {
	Op  deploy.StepOp
	URN resource.URN
}

func AssertSameSteps(t *testing.T, expected []StepSummary, actual []deploy.Step) bool {
	assert.Equal(t, len(expected), len(actual))
	for _, exp := range expected {
		act := actual[0]
		actual = actual[1:]

		if !assert.Equal(t, exp.Op, act.Op()) || !assert.Equal(t, exp.URN, act.URN()) {
			return false
		}
	}
	return true
}

func TestEmptyProgramLifecycle(t *testing.T) {
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 0),
	}
	p.Run(t, nil)
}

func TestSingleResourceDefaultProviderLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)
}

func TestSingleResourceExplicitProviderLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)
}

func TestSingleResourceDefaultProviderUpgrade(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with an existing copy of the single resource and no providers.
	old := &deploy.Snapshot{
		Resources: []*resource.State{{
			Type:    resURN.Type(),
			URN:     resURN,
			Custom:  true,
			ID:      "0",
			Inputs:  resource.PropertyMap{},
			Outputs: resource.PropertyMap{},
		}},
	}

	isRefresh := false
	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		_ []Event, res result.Result) result.Result {

		// Should see only sames: the default provider should be injected into the old state before the update
		// runs.
		for _, entry := range entries {
			switch urn := entry.Step.URN(); urn {
			case provURN, resURN:
				expect := deploy.OpSame
				if isRefresh {
					expect = deploy.OpRefresh
				}
				assert.Equal(t, expect, entry.Step.Op())
			default:
				t.Fatalf("unexpected resource %v", urn)
			}
		}
		assert.Len(t, entries.Snap(target.Snapshot).Resources, 2)
		return res
	}

	// Run a single update step using the base snapshot.
	p.Steps = []TestStep{{Op: Update, Validate: validate}}
	p.Run(t, old)

	// Run a single refresh step using the base snapshot.
	isRefresh = true
	p.Steps = []TestStep{{Op: Refresh, Validate: validate}}
	p.Run(t, old)

	// Run a single destroy step using the base snapshot.
	isRefresh = false
	p.Steps = []TestStep{{
		Op: Destroy,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			// Should see two deletes:  the default provider should be injected into the old state before the update
			// runs.
			deleted := make(map[resource.URN]bool)
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					deleted[urn] = true
					assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.Len(t, deleted, 2)
			assert.Len(t, entries.Snap(target.Snapshot).Resources, 0)
			return res
		},
	}}
	p.Run(t, old)

	// Run a partial lifecycle using the base snapshot, skipping the initial update step.
	p.Steps = MakeBasicLifecycleSteps(t, 2)[1:]
	p.Run(t, old)
}

func TestSingleResourceDefaultProviderReplace(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range news {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Config: config.Map{
			config.MustMakeKey("pkgA", "foo"): config.NewValue("bar"),
		},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its no-op update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// Change the config and run an update. We expect everything to require replacement.
	p.Config[config.MustMakeKey("pkgA", "foo")] = config.NewValue("baz")
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			provURN := p.NewProviderURN("pkgA", "default", "")
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			replacedProvider, replacedResource := false, false
			for _, entry := range entries {
				if entry.Kind != JournalEntrySuccess || entry.Step.Op() != deploy.OpDeleteReplaced {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case provURN:
					replacedProvider = true
				case resURN:
					replacedResource = true
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, replacedProvider)
			assert.True(t, replacedResource)

			return res
		},
	}}

	snap = p.Run(t, snap)

	// Resume the lifecycle with another no-op update.
	p.Steps = steps[2:]
	p.Run(t, snap)
}

func TestSingleResourceExplicitProviderReplace(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range news {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys}, nil
				},
			}, nil
		}),
	}

	providerInputs := resource.PropertyMap{
		resource.PropertyKey("foo"): resource.NewStringProperty("bar"),
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true,
			deploytest.ResourceOptions{Inputs: providerInputs})
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its no-op update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// Change the config and run an update. We expect everything to require replacement.
	providerInputs[resource.PropertyKey("foo")] = resource.NewStringProperty("baz")
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			provURN := p.NewProviderURN("pkgA", "provA", "")
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			replacedProvider, replacedResource := false, false
			for _, entry := range entries {
				if entry.Kind != JournalEntrySuccess || entry.Step.Op() != deploy.OpDeleteReplaced {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case provURN:
					replacedProvider = true
				case resURN:
					replacedResource = true
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, replacedProvider)
			assert.True(t, replacedResource)

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Resume the lifecycle with another no-op update.
	p.Steps = steps[2:]
	p.Run(t, snap)
}

func TestSingleResourceExplicitProviderDeleteBeforeReplace(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range news {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys, DeleteBeforeReplace: true}, nil
				},
			}, nil
		}),
	}

	providerInputs := resource.PropertyMap{
		resource.PropertyKey("foo"): resource.NewStringProperty("bar"),
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true,
			deploytest.ResourceOptions{Inputs: providerInputs})
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its no-op update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// Change the config and run an update. We expect everything to require replacement.
	providerInputs[resource.PropertyKey("foo")] = resource.NewStringProperty("baz")
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			provURN := p.NewProviderURN("pkgA", "provA", "")
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			createdProvider, createdResource := false, false
			deletedProvider, deletedResource := false, false
			for _, entry := range entries {
				if entry.Kind != JournalEntrySuccess {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case provURN:
					if entry.Step.Op() == deploy.OpDeleteReplaced {
						assert.False(t, createdProvider)
						assert.False(t, createdResource)
						assert.True(t, deletedResource)
						deletedProvider = true
					} else if entry.Step.Op() == deploy.OpCreateReplacement {
						assert.True(t, deletedProvider)
						assert.True(t, deletedResource)
						assert.False(t, createdResource)
						createdProvider = true
					}
				case resURN:
					if entry.Step.Op() == deploy.OpDeleteReplaced {
						assert.False(t, deletedProvider)
						assert.False(t, deletedResource)
						deletedResource = true
					} else if entry.Step.Op() == deploy.OpCreateReplacement {
						assert.True(t, deletedProvider)
						assert.True(t, deletedResource)
						assert.True(t, createdProvider)
						createdResource = true
					}
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, deletedProvider)
			assert.True(t, deletedResource)

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Resume the lifecycle with another no-op update.
	p.Steps = steps[2:]
	p.Run(t, snap)
}

func TestSingleResourceDiffUnavailable(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					return plugin.DiffResult{}, plugin.DiffUnavailable("diff unavailable")
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// Now change the inputs to our resource and run a preview.
	inputs = resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

			found := false
			for _, e := range events {
				if e.Type == DiagEvent {
					p := e.Payload().(DiagEventPayload)
					if p.URN == resURN && p.Severity == diag.Warning && p.Message == "diff unavailable" {
						found = true
						break
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)
}

func TestDestroyWithPendingDelete(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(_ workspace.Project, _ deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resource with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != JournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return res
		},
	}}
	p.Run(t, old)
}

func TestUpdateWithPendingDelete(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	host := deploytest.NewPluginHost(nil, nil, nil, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []TestStep{{
		Op: Destroy,
		Validate: func(_ workspace.Project, _ deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resource with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != JournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return res
		},
	}}
	p.Run(t, old)
}

func TestParallelRefresh(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Create a program that registers four resources, each of which depends on the resource that immediately precedes
	// it.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resA},
		})
		assert.NoError(t, err)

		resC, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resB},
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resC},
		})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Parallel: 4, Host: host},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.Equal(t, string(snap.Resources[2].URN.Name()), "resB")
	assert.Equal(t, string(snap.Resources[3].URN.Name()), "resC")
	assert.Equal(t, string(snap.Resources[4].URN.Name()), "resD")

	p.Steps = []TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.Equal(t, string(snap.Resources[2].URN.Name()), "resB")
	assert.Equal(t, string(snap.Resources[3].URN.Name()), "resC")
	assert.Equal(t, string(snap.Resources[4].URN.Name()), "resD")
}

func TestExternalRefresh(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Our program reads a resource and exits.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "", "")
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update}},
	}

	// The read should place "resA" in the snapshot with the "External" bit set.
	snap := p.Run(t, nil)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.True(t, snap.Resources[1].External)

	p = &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Refresh}},
	}

	snap = p.Run(t, snap)
	// A refresh should leave "resA" as it is in the snapshot. The External bit should still be set.
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.True(t, snap.Resources[1].External)
}

func TestRefreshInitFailure(t *testing.T) {
	p := &TestPlan{}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	res2URN := p.NewURN("pkgA:m:typA", "resB", "")

	res2Outputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}

	//
	// Refresh will persist any initialization errors that are returned by `Read`. This provider
	// will error out or not based on the value of `refreshShouldFail`.
	//
	refreshShouldFail := false

	//
	// Set up test environment to use `readFailProvider` as the underlying resource provider.
	//
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					if refreshShouldFail && urn == resURN {
						err := &plugin.InitError{
							Reasons: []string{"Refresh reports continued to fail to initialize"},
						}
						return plugin.ReadResult{Outputs: resource.PropertyMap{}}, resource.StatusPartialFailure, err
					} else if urn == res2URN {
						return plugin.ReadResult{Outputs: res2Outputs}, resource.StatusOK, nil
					}
					return plugin.ReadResult{Outputs: resource.PropertyMap{}}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Options.Host = host

	//
	// Create an old snapshot with a single initialization failure.
	//
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:       resURN.Type(),
				URN:        resURN,
				Custom:     true,
				ID:         "0",
				Inputs:     resource.PropertyMap{},
				Outputs:    resource.PropertyMap{},
				InitErrors: []string{"Resource failed to initialize"},
			},
			{
				Type:    res2URN.Type(),
				URN:     res2URN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
		},
	}

	//
	// Refresh DOES NOT fail, causing the initialization error to disappear.
	//
	p.Steps = []TestStep{{Op: Refresh}}
	snap := p.Run(t, old)

	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Empty(t, resource.InitErrors)
		case res2URN:
			assert.Equal(t, res2Outputs, resource.Outputs)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}

	//
	// Refresh again, see the resource is in a partial state of failure, but the refresh operation
	// DOES NOT fail. The initialization error is still persisted.
	//
	refreshShouldFail = true
	p.Steps = []TestStep{{Op: Refresh, SkipPreview: true}}
	snap = p.Run(t, old)
	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Equal(t, []string{"Refresh reports continued to fail to initialize"}, resource.InitErrors)
		case res2URN:
			assert.Equal(t, res2Outputs, resource.Outputs)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}
}

// Test that ensures that we log diagnostics for resources that receive an error from Check. (Note that this
// is distinct from receiving non-error failures from Check.)
func TestCheckFailureRecord(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(urn resource.URN,
					olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return nil, nil, errors.New("oh no, check had an error")
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result) result.Result {

				sawFailure := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						e := evt.Payload().(DiagEventPayload)
						msg := colors.Never.Colorize(e.Message)
						sawFailure = msg == "oh no, check had an error\n" && e.Severity == diag.Error
					}
				}

				assert.True(t, sawFailure)
				return res
			},
		}},
	}

	p.Run(t, nil)
}

// Test that checks that we emit diagnostics for properties that check says are invalid.
func TestCheckFailureInvalidPropertyRecord(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(urn resource.URN,
					olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return nil, []plugin.CheckFailure{{
						Property: "someprop",
						Reason:   "field is not valid",
					}}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result) result.Result {

				sawFailure := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						e := evt.Payload().(DiagEventPayload)
						msg := colors.Never.Colorize(e.Message)
						sawFailure = strings.Contains(msg, "field is not valid") && e.Severity == diag.Error
						if sawFailure {
							break
						}
					}
				}

				assert.True(t, sawFailure)
				return res
			},
		}},
	}

	p.Run(t, nil)

}

// Test that tests that Refresh can detect that resources have been deleted and removes them
// from the snapshot.
func TestRefreshWithDelete(t *testing.T) {
	for _, parallelFactor := range []int{1, 4} {
		t.Run(fmt.Sprintf("parallel-%d", parallelFactor), func(t *testing.T) {
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						ReadF: func(
							urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
						) (plugin.ReadResult, resource.Status, error) {
							// This thing doesn't exist. Returning nil from Read should trigger
							// the engine to delete it from the snapshot.
							return plugin.ReadResult{}, resource.StatusOK, nil
						},
					}, nil
				}),
			}

			program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
				assert.NoError(t, err)
				return err
			})

			host := deploytest.NewPluginHost(nil, nil, program, loaders...)
			p := &TestPlan{Options: UpdateOptions{Host: host, Parallel: parallelFactor}}

			p.Steps = []TestStep{{Op: Update}}
			snap := p.Run(t, nil)

			p.Steps = []TestStep{{Op: Refresh}}
			snap = p.Run(t, snap)

			// Refresh succeeds and records that the resource in the snapshot doesn't exist anymore
			provURN := p.NewProviderURN("pkgA", "default", "")
			assert.Len(t, snap.Resources, 1)
			assert.Equal(t, provURN, snap.Resources[0].URN)
		})
	}
}

func pickURN(t *testing.T, urns []resource.URN, names []string, target string) resource.URN {
	assert.Equal(t, len(urns), len(names))
	assert.Contains(t, names, target)

	for i, name := range names {
		if name == target {
			return urns[i]
		}
	}

	t.Fatalf("Could not find target: %v in %v", target, names)
	return ""
}

// Tests that dependencies are correctly rewritten when refresh removes deleted resources.
func TestRefreshDeleteDependencies(t *testing.T) {
	names := []string{"resA", "resB", "resC"}

	// Try refreshing a stack with every combination of the three above resources as a target to
	// refresh.
	subsets := combinations.All(names)

	// combinations.All doesn't return the empty set.  So explicitly test that case (i.e. test no
	// targets specified)
	validateRefreshDeleteCombination(t, names, []string{})

	for _, subset := range subsets {
		validateRefreshDeleteCombination(t, names, subset)
	}
}

func validateRefreshDeleteCombination(t *testing.T, names []string, targets []string) {
	p := &TestPlan{}

	const resType = "pkgA:m:typA"

	urnA := p.NewURN(resType, names[0], "")
	urnB := p.NewURN(resType, names[1], "")
	urnC := p.NewURN(resType, names[2], "")
	urns := []resource.URN{urnA, urnB, urnC}

	refreshTargets := []resource.URN{}

	t.Logf("Refreshing targets: %v", targets)
	for _, target := range targets {
		refreshTargets = append(refreshTargets, pickURN(t, urns, names, target))
	}

	p.Options.RefreshTargets = refreshTargets

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	oldResources := []*resource.State{
		newResource(urnA, "0", false),
		newResource(urnB, "1", false, urnA),
		newResource(urnC, "2", false, urnA, urnB),
		newResource(urnA, "3", true),
		newResource(urnA, "4", true),
		newResource(urnC, "5", true, urnA, urnB),
	}

	old := &deploy.Snapshot{
		Resources: oldResources,
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					switch id {
					case "0", "4":
						// We want to delete resources A::0 and A::4.
						return plugin.ReadResult{}, resource.StatusOK, nil
					default:
						return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
					}
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, nil, loaders...)

	p.Steps = []TestStep{
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				// Should see only refreshes.
				for _, entry := range entries {
					if len(refreshTargets) > 0 {
						// should only see changes to urns we explicitly asked to change
						assert.Containsf(t, refreshTargets, entry.Step.URN(),
							"Refreshed a resource that wasn't a target: %v", entry.Step.URN())
					}

					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
				}

				return res
			},
		},
	}

	snap := p.Run(t, old)

	provURN := p.NewProviderURN("pkgA", "default", "")

	for _, r := range snap.Resources {
		switch urn := r.URN; urn {
		case provURN:
			continue
		case urnA, urnB, urnC:
			// break
		default:
			t.Fatalf("unexpected resource %v", urn)
		}

		if len(refreshTargets) == 0 || containsURN(refreshTargets, urnA) {
			// 'A' was deleted, so we should see the impact downstream.

			switch r.ID {
			case "1":
				// A::0 was deleted, so B's dependency list should be empty.
				assert.Equal(t, urnB, r.URN)
				assert.Empty(t, r.Dependencies)
			case "2":
				// A::0 was deleted, so C's dependency list should only contain B.
				assert.Equal(t, urnC, r.URN)
				assert.Equal(t, []resource.URN{urnB}, r.Dependencies)
			case "3":
				// A::3 should not have changed.
				assert.Equal(t, oldResources[3], r)
			case "5":
				// A::4 was deleted but A::3 was still refernceable by C, so C should not have changed.
				assert.Equal(t, oldResources[5], r)
			default:
				t.Fatalf("Unexpected changed resource when refreshing %v: %v::%v", refreshTargets, r.URN, r.ID)
			}
		} else {
			// A was not deleted. So nothing should be impacted.
			id, err := strconv.Atoi(r.ID.String())
			assert.NoError(t, err)
			assert.Equal(t, oldResources[id], r)
		}
	}
}

func containsURN(urns []resource.URN, urn resource.URN) bool {
	for _, val := range urns {
		if val == urn {
			return true
		}
	}

	return false
}

// Tests basic refresh functionality.
func TestRefreshBasics(t *testing.T) {
	names := []string{"resA", "resB", "resC"}

	// Try refreshing a stack with every combination of the three above resources as a target to
	// refresh.
	subsets := combinations.All(names)

	// combinations.All doesn't return the empty set.  So explicitly test that case (i.e. test no
	// targets specified)
	validateRefreshBasicsCombination(t, names, []string{})

	for _, subset := range subsets {
		validateRefreshBasicsCombination(t, names, subset)
	}
}

func validateRefreshBasicsCombination(t *testing.T, names []string, targets []string) {
	p := &TestPlan{}

	const resType = "pkgA:m:typA"

	urnA := p.NewURN(resType, names[0], "")
	urnB := p.NewURN(resType, names[1], "")
	urnC := p.NewURN(resType, names[2], "")
	urns := []resource.URN{urnA, urnB, urnC}

	refreshTargets := []resource.URN{}

	for _, target := range targets {
		refreshTargets = append(p.Options.RefreshTargets, pickURN(t, urns, names, target))
	}

	p.Options.RefreshTargets = refreshTargets

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	oldResources := []*resource.State{
		newResource(urnA, "0", false),
		newResource(urnB, "1", false, urnA),
		newResource(urnC, "2", false, urnA, urnB),
		newResource(urnA, "3", true),
		newResource(urnA, "4", true),
		newResource(urnC, "5", true, urnA, urnB),
	}

	newStates := map[resource.ID]plugin.ReadResult{
		// A::0 and A::3 will have no changes.
		"0": {Outputs: resource.PropertyMap{}, Inputs: resource.PropertyMap{}},
		"3": {Outputs: resource.PropertyMap{}, Inputs: resource.PropertyMap{}},

		// B::1 and A::4 will have changes. The latter will also have input changes.
		"1": {Outputs: resource.PropertyMap{"foo": resource.NewStringProperty("bar")}, Inputs: resource.PropertyMap{}},
		"4": {
			Outputs: resource.PropertyMap{"baz": resource.NewStringProperty("qux")},
			Inputs:  resource.PropertyMap{"oof": resource.NewStringProperty("zab")},
		},

		// C::2 and C::5 will be deleted.
		"2": {},
		"5": {},
	}

	old := &deploy.Snapshot{
		Resources: oldResources,
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					new, hasNewState := newStates[id]
					assert.True(t, hasNewState)
					return new, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, nil, loaders...)

	p.Steps = []TestStep{{
		Op: Refresh,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			// Should see only refreshes.
			for _, entry := range entries {
				if len(refreshTargets) > 0 {
					// should only see changes to urns we explicitly asked to change
					assert.Containsf(t, refreshTargets, entry.Step.URN(),
						"Refreshed a resource that wasn't a target: %v", entry.Step.URN())
				}

				assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
				resultOp := entry.Step.(*deploy.RefreshStep).ResultOp()

				old := entry.Step.Old()
				if !old.Custom || providers.IsProviderType(old.Type) {
					// Component and provider resources should never change.
					assert.Equal(t, deploy.OpSame, resultOp)
					continue
				}

				expected, new := newStates[old.ID], entry.Step.New()
				if expected.Outputs == nil {
					// If the resource was deleted, we want the result op to be an OpDelete.
					assert.Nil(t, new)
					assert.Equal(t, deploy.OpDelete, resultOp)
				} else {
					// If there were changes to the outputs, we want the result op to be an OpUpdate. Otherwise we want
					// an OpSame.
					if reflect.DeepEqual(old.Outputs, expected.Outputs) {
						assert.Equal(t, deploy.OpSame, resultOp)
					} else {
						assert.Equal(t, deploy.OpUpdate, resultOp)
					}

					// Only the inputs and outputs should have changed (if anything changed).
					old.Inputs = expected.Inputs
					old.Outputs = expected.Outputs
					assert.Equal(t, old, new)
				}
			}
			return res
		},
	}}
	snap := p.Run(t, old)

	provURN := p.NewProviderURN("pkgA", "default", "")

	for _, r := range snap.Resources {
		switch urn := r.URN; urn {
		case provURN:
			continue
		case urnA, urnB, urnC:
			// break
		default:
			t.Fatalf("unexpected resource %v", urn)
		}

		// The only resources left in the checkpoint should be those that were not deleted by the refresh.
		expected := newStates[r.ID]
		assert.NotNil(t, expected)

		idx, err := strconv.ParseInt(string(r.ID), 0, 0)
		assert.NoError(t, err)

		// The new resources should be equal to the old resources + the new inputs and outputs.
		old := oldResources[int(idx)]
		old.Inputs = expected.Inputs
		old.Outputs = expected.Outputs
		assert.Equal(t, old, r)
	}
}

// Tests that an interrupted refresh leaves behind an expected state.
func TestCanceledRefresh(t *testing.T) {
	p := &TestPlan{}

	const resType = "pkgA:m:typA"

	urnA := p.NewURN(resType, "resA", "")
	urnB := p.NewURN(resType, "resB", "")
	urnC := p.NewURN(resType, "resC", "")

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	oldResources := []*resource.State{
		newResource(urnA, "0", false),
		newResource(urnB, "1", false),
		newResource(urnC, "2", false),
	}

	newStates := map[resource.ID]resource.PropertyMap{
		// A::0 and B::1 will have changes; D::3 will be deleted.
		"0": {"foo": resource.NewStringProperty("bar")},
		"1": {"baz": resource.NewStringProperty("qux")},
		"2": nil,
	}

	old := &deploy.Snapshot{
		Resources: oldResources,
	}

	// Set up a cancelable context for the refresh operation.
	ctx, cancel := context.WithCancel(context.Background())

	// Serialize all refreshes s.t. we can cancel after the first is issued.
	refreshes, cancelled := make(chan resource.ID), make(chan bool)
	go func() {
		<-refreshes
		cancel()
	}()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					refreshes <- id
					<-cancelled

					new, hasNewState := newStates[id]
					assert.True(t, hasNewState)
					return plugin.ReadResult{Outputs: new}, resource.StatusOK, nil
				},
				CancelF: func() error {
					close(cancelled)
					return nil
				},
			}, nil
		}),
	}

	refreshed := make(map[resource.ID]bool)
	op := TestOp(Refresh)
	options := UpdateOptions{
		Parallel: 1,
		Host:     deploytest.NewPluginHost(nil, nil, nil, loaders...),
	}
	project, target := p.GetProject(), p.GetTarget(old)
	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		_ []Event, res result.Result) result.Result {

		for _, entry := range entries {
			assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
			resultOp := entry.Step.(*deploy.RefreshStep).ResultOp()

			old := entry.Step.Old()
			if !old.Custom || providers.IsProviderType(old.Type) {
				// Component and provider resources should never change.
				assert.Equal(t, deploy.OpSame, resultOp)
				continue
			}

			refreshed[old.ID] = true

			expected, new := newStates[old.ID], entry.Step.New()
			if expected == nil {
				// If the resource was deleted, we want the result op to be an OpDelete.
				assert.Nil(t, new)
				assert.Equal(t, deploy.OpDelete, resultOp)
			} else {
				// If there were changes to the outputs, we want the result op to be an OpUpdate. Otherwise we want
				// an OpSame.
				if reflect.DeepEqual(old.Outputs, expected) {
					assert.Equal(t, deploy.OpSame, resultOp)
				} else {
					assert.Equal(t, deploy.OpUpdate, resultOp)
				}

				// Only the outputs should have changed (if anything changed).
				old.Outputs = expected
				assert.Equal(t, old, new)
			}
		}
		return res
	}

	snap, res := op.RunWithContext(ctx, project, target, options, false, nil, validate)
	assertIsErrorOrBailResult(t, res)
	assert.Equal(t, 1, len(refreshed))

	provURN := p.NewProviderURN("pkgA", "default", "")

	for _, r := range snap.Resources {
		switch urn := r.URN; urn {
		case provURN:
			continue
		case urnA, urnB, urnC:
			// break
		default:
			t.Fatalf("unexpected resource %v", urn)
		}

		idx, err := strconv.ParseInt(string(r.ID), 0, 0)
		assert.NoError(t, err)

		if refreshed[r.ID] {
			// The refreshed resource should have its new state.
			expected := newStates[r.ID]
			if expected == nil {
				assert.Fail(t, "refreshed resource was not deleted")
			} else {
				old := oldResources[int(idx)]
				old.Outputs = expected
				assert.Equal(t, old, r)
			}
		} else {
			// Any resources that were not refreshed should retain their original state.
			old := oldResources[int(idx)]
			assert.Equal(t, old, r)
		}
	}
}

// Tests that errors returned directly from the language host get logged by the engine.
func TestLanguageHostDiagnostics(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	errorText := "oh no"
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		// Exiting immediately with an error simulates a language exiting immediately with a non-zero exit code.
		return errors.New(errorText)
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result) result.Result {

				assertIsErrorOrBailResult(t, res)
				sawExitCode := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						e := evt.Payload().(DiagEventPayload)
						msg := colors.Never.Colorize(e.Message)
						sawExitCode = strings.Contains(msg, errorText) && e.Severity == diag.Error
						if sawExitCode {
							break
						}
					}
				}

				assert.True(t, sawExitCode)
				return res
			},
		}},
	}

	p.Run(t, nil)
}

type brokenDecrypter struct {
	ErrorMessage string
}

func (b brokenDecrypter) DecryptValue(ciphertext string) (string, error) {
	return "", fmt.Errorf(b.ErrorMessage)
}

// Tests that the engine presents a reasonable error message when a decrypter fails to decrypt a config value.
func TestBrokenDecrypter(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	key := config.MustMakeKey("foo", "bar")
	msg := "decryption failed"
	configMap := make(config.Map)
	configMap[key] = config.NewSecureValue("hunter2")
	p := &TestPlan{
		Options:   UpdateOptions{Host: host},
		Decrypter: brokenDecrypter{ErrorMessage: msg},
		Config:    configMap,
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result) result.Result {

				assertIsErrorOrBailResult(t, res)
				decryptErr := res.Error().(DecryptError)
				assert.Equal(t, key, decryptErr.Key)
				assert.Contains(t, decryptErr.Err.Error(), msg)
				return res
			},
		}},
	}

	p.Run(t, nil)
}

func TestBadResourceType(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("very:bad", "resA", true)
		assert.Error(t, err)
		rpcerr, ok := rpcerror.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, rpcerr.Code())
		assert.Contains(t, rpcerr.Message(), "Type 'very:bad' is not a valid type token")

		_, _, err = mon.ReadResource("very:bad", "someResource", "someId", "", resource.PropertyMap{}, "", "")
		assert.Error(t, err)
		rpcerr, ok = rpcerror.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, rpcerr.Code())
		assert.Contains(t, rpcerr.Message(), "Type 'very:bad' is not a valid type token")

		// Component resources may have any format type.
		_, _, _, noErr := mon.RegisterResource("a:component", "resB", false)
		assert.NoError(t, noErr)

		_, _, _, noErr = mon.RegisterResource("singlename", "resC", false)
		assert.NoError(t, noErr)

		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
		}},
	}

	p.Run(t, nil)
}

// Tests that provider cancellation occurs as expected.
func TestProviderCancellation(t *testing.T) {
	const resourceCount = 4

	// Set up a cancelable context for the refresh operation.
	ctx, cancel := context.WithCancel(context.Background())

	// Wait for our resource ops, then cancel.
	var ops sync.WaitGroup
	ops.Add(resourceCount)
	go func() {
		ops.Wait()
		cancel()
	}()

	// Set up an independent cancelable context for the provider's operations.
	provCtx, provCancel := context.WithCancel(context.Background())
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					// Inform the waiter that we've entered a provider op and wait for cancellation.
					ops.Done()
					<-provCtx.Done()

					return resource.ID(urn.Name()), resource.PropertyMap{}, resource.StatusOK, nil
				},
				CancelF: func() error {
					provCancel()
					return nil
				},
			}, nil
		}),
	}

	done := make(chan bool)
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		errors := make([]error, resourceCount)
		var resources sync.WaitGroup
		resources.Add(resourceCount)
		for i := 0; i < resourceCount; i++ {
			go func(idx int) {
				_, _, _, errors[idx] = monitor.RegisterResource("pkgA:m:typA", fmt.Sprintf("res%d", idx), true)
				resources.Done()
			}(i)
		}
		resources.Wait()
		for _, err := range errors {
			assert.NoError(t, err)
		}
		close(done)
		return nil
	})

	p := &TestPlan{}
	op := TestOp(Update)
	options := UpdateOptions{
		Parallel: resourceCount,
		Host:     deploytest.NewPluginHost(nil, nil, program, loaders...),
	}
	project, target := p.GetProject(), p.GetTarget(nil)

	_, res := op.RunWithContext(ctx, project, target, options, false, nil, nil)
	assertIsErrorOrBailResult(t, res)

	// Wait for the program to finish.
	<-done
}

// Tests that a preview works for a stack with pending operations.
func TestPreviewWithPendingOperations(t *testing.T) {
	p := &TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	old := &deploy.Snapshot{
		PendingOperations: []resource.Operation{{
			Resource: newResource(urnA, "0", false),
			Type:     resource.OperationTypeUpdating,
		}},
		Resources: []*resource.State{
			newResource(urnA, "0", false),
		},
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := TestOp(Update)
	options := UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	project, target := p.GetProject(), p.GetTarget(old)

	// A preview should succeed despite the pending operations.
	_, res := op.Run(project, target, options, true, nil, nil)
	assert.Nil(t, res)

	// But an update should fail.
	_, res = op.Run(project, target, options, false, nil, nil)
	assertIsErrorOrBailResult(t, res)
	assert.EqualError(t, res.Error(), deploy.PlanPendingOperationsError{}.Error())
}

// Tests that a failed partial update causes the engine to persist the resource's old inputs and new outputs.
func TestUpdatePartialFailure(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"output_prop": 42,
					})

					return outputs, resource.StatusPartialFailure, errors.New("update failed to apply")
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"input_prop": "new inputs",
			}),
		})
		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{Options: UpdateOptions{Host: host}}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
		SkipPreview:   true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assertIsErrorOrBailResult(t, res)
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case resURN:
					assert.Equal(t, deploy.OpUpdate, entry.Step.Op())
					switch entry.Kind {
					case JournalEntryBegin:
						continue
					case JournalEntrySuccess:
						inputs := entry.Step.New().Inputs
						outputs := entry.Step.New().Outputs
						assert.Len(t, inputs, 1)
						assert.Len(t, outputs, 1)
						assert.Equal(t,
							resource.NewStringProperty("old inputs"), inputs[resource.PropertyKey("input_prop")])
						assert.Equal(t,
							resource.NewNumberProperty(42), outputs[resource.PropertyKey("output_prop")])
					default:
						t.Fatalf("unexpected journal operation: %d", entry.Kind)
					}
				}
			}

			return res
		},
	}}

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   resURN.Type(),
				URN:    resURN,
				Custom: true,
				ID:     "1",
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"input_prop": "old inputs",
				}),
				Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"output_prop": 1,
				}),
			},
		},
	}

	p.Run(t, old)
}

// Tests that the StackReference resource works as intended,
func TestStackReference(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{}

	// Test that the normal lifecycle works correctly.
	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, state, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
			}),
		})
		assert.NoError(t, err)
		if !info.DryRun {
			assert.Equal(t, "bar", state["outputs"].ObjectValue()["foo"].StringValue())
		}
		return nil
	})
	p := &TestPlan{
		BackendClient: &deploytest.BackendClient{
			GetStackOutputsF: func(ctx context.Context, name string) (resource.PropertyMap, error) {
				switch name {
				case "other":
					return resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo": "bar",
					}), nil
				default:
					return nil, errors.Errorf("unknown stack \"%s\"", name)
				}
			},
		},
		Options: UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)

	// Test that changes to `name` cause replacement.
	resURN := p.NewURN("pulumi:pulumi:StackReference", "other", "")
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   resURN.Type(),
				URN:    resURN,
				Custom: true,
				ID:     "1",
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"name": "other2",
				}),
				Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"name":    "other2",
					"outputs": resource.PropertyMap{},
				}),
			},
		},
	}
	p.Steps = []TestStep{{
		Op:          Update,
		SkipPreview: true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case resURN:
					switch entry.Step.Op() {
					case deploy.OpCreateReplacement, deploy.OpDeleteReplaced, deploy.OpReplace:
						// OK
					default:
						t.Fatalf("unexpected journal operation: %v", entry.Step.Op())
					}
				}
			}

			return res
		},
	}}
	p.Run(t, old)

	// Test that unknown stacks are handled appropriately.
	program = deploytest.NewLanguageRuntime(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "rehto",
			}),
		})
		assert.Error(t, err)
		return err
	})
	p.Options = UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
		SkipPreview:   true,
	}}
	p.Run(t, nil)

	// Test that unknown properties cause errors.
	program = deploytest.NewLanguageRuntime(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
				"foo":  "bar",
			}),
		})
		assert.Error(t, err)
		return err
	})
	p.Options = UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	p.Run(t, nil)
}

type channelWriter struct {
	channel chan []byte
}

func (cw *channelWriter) Write(d []byte) (int, error) {
	cw.channel <- d
	return len(d), nil
}

// Tests that a failed plugin load correctly shuts down the host.
func TestLoadFailureShutdown(t *testing.T) {

	// Note that the setup here is a bit baroque, and is intended to replicate the CLI architecture that lead to
	// issue #2170. That issue--a panic on a closed channel--was caused by the intersection of several design choices:
	//
	// - The provider registry loads and configures the set of providers necessary for the resources currently in the
	//   checkpoint it is processing at plan creation time. Registry creation fails promptly if a provider plugin
	//   fails to load (e.g. because is binary is missing).
	// - Provider configuration in the CLI's host happens asynchronously. This is meant to allow the engine to remain
	//   responsive while plugins configure.
	// - Providers may call back into the CLI's host for logging. Callbacks are processed as long as the CLI's plugin
	//   context is open.
	// - Log events from the CLI's host are delivered to the CLI's diagnostic streams via channels. The CLI closes
	//   these channels once the engine operation it initiated completes.
	//
	// These choices gave rise to the following situation:
	// 1. The provider registry loads a provider for package A and kicks off its configuration.
	// 2. The provider registry attempts to load a provider for package B. The load fails, and the provider registry
	//   creation call fails promptly.
	// 3. The engine operation requested by the CLI fails promptly because provider registry creation failed.
	// 4. The CLI shuts down its diagnostic channels.
	// 5. The provider for package A calls back in to the host to log a message. The host then attempts to deliver
	//    the message to the CLI's diagnostic channels, causing a panic.
	//
	// The fix was to properly close the plugin host during step (3) s.t. the host was no longer accepting callbacks
	// and would not attempt to send messages to the CLI's diagnostic channels.
	//
	// As such, this test attempts to replicate the CLI architecture by using one provider that configures
	// asynchronously and attempts to call back into the engine and a second provider that fails to load.

	release, done := make(chan bool), make(chan bool)
	sinkWriter := &channelWriter{channel: make(chan []byte)}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoaderWithHost("pkgA", semver.MustParse("1.0.0"),
			func(host plugin.Host) (plugin.Provider, error) {
				return &deploytest.Provider{
					ConfigureF: func(news resource.PropertyMap) error {
						go func() {
							<-release
							host.Log(diag.Info, "", "configuring pkgA provider...", 0)
							close(done)
						}()
						return nil
					},
				}, nil
			}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return nil, errors.New("pkgB load failure")
		}),
	}

	p := &TestPlan{}
	provAURN := p.NewProviderURN("pkgA", "default", "")
	provBURN := p.NewProviderURN("pkgB", "default", "")

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    provAURN.Type(),
				URN:     provAURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    provBURN.Type(),
				URN:     provBURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
		},
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	op := TestOp(Update)
	sink := diag.DefaultSink(sinkWriter, sinkWriter, diag.FormatOptions{Color: colors.Raw})
	options := UpdateOptions{Host: deploytest.NewPluginHost(sink, sink, program, loaders...)}
	project, target := p.GetProject(), p.GetTarget(old)

	_, res := op.Run(project, target, options, true, nil, nil)
	assertIsErrorOrBailResult(t, res)

	close(sinkWriter.channel)
	close(release)
	<-done
}

var complexTestDependencyGraphNames = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"}

func generateComplexTestDependencyGraph(
	t *testing.T, p *TestPlan) ([]resource.URN, *deploy.Snapshot, plugin.LanguageRuntime) {

	resType := tokens.Type("pkgA:m:typA")
	type propertyDependencies map[resource.PropertyKey][]resource.URN

	names := complexTestDependencyGraphNames

	urnA := p.NewProviderURN("pkgA", names[0], "")
	urnB := p.NewURN(resType, names[1], "")
	urnC := p.NewProviderURN("pkgA", names[2], "")
	urnD := p.NewProviderURN("pkgA", names[3], "")
	urnE := p.NewURN(resType, names[4], "")
	urnF := p.NewURN(resType, names[5], "")
	urnG := p.NewURN(resType, names[6], "")
	urnH := p.NewURN(resType, names[7], "")
	urnI := p.NewURN(resType, names[8], "")
	urnJ := p.NewURN(resType, names[9], "")
	urnK := p.NewURN(resType, names[10], "")
	urnL := p.NewURN(resType, names[11], "")

	urns := []resource.URN{
		urnA, urnB, urnC, urnD, urnE, urnF,
		urnG, urnH, urnI, urnJ, urnK, urnL,
	}

	newResource := func(urn resource.URN, id resource.ID, provider string, dependencies []resource.URN,
		propertyDeps propertyDependencies, outputs resource.PropertyMap) *resource.State {

		inputs := resource.PropertyMap{}
		for k := range propertyDeps {
			inputs[k] = resource.NewStringProperty("foo")
		}

		return &resource.State{
			Type:                 urn.Type(),
			URN:                  urn,
			Custom:               true,
			Delete:               false,
			ID:                   id,
			Inputs:               inputs,
			Outputs:              outputs,
			Dependencies:         dependencies,
			Provider:             provider,
			PropertyDependencies: propertyDeps,
		}
	}

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			newResource(urnA, "0", "", nil, nil, resource.PropertyMap{"A": resource.NewStringProperty("foo")}),
			newResource(urnB, "1", string(urnA)+"::0", nil, nil, nil),
			newResource(urnC, "2", "",
				[]resource.URN{urnA},
				propertyDependencies{"A": []resource.URN{urnA}},
				resource.PropertyMap{"A": resource.NewStringProperty("bar")}),
			newResource(urnD, "3", "",
				[]resource.URN{urnA},
				propertyDependencies{"B": []resource.URN{urnA}}, nil),
			newResource(urnE, "4", string(urnC)+"::2", nil, nil, nil),
			newResource(urnF, "5", "",
				[]resource.URN{urnC},
				propertyDependencies{"A": []resource.URN{urnC}}, nil),
			newResource(urnG, "6", "",
				[]resource.URN{urnC},
				propertyDependencies{"B": []resource.URN{urnC}}, nil),
			newResource(urnH, "4", string(urnD)+"::3", nil, nil, nil),
			newResource(urnI, "5", "",
				[]resource.URN{urnD},
				propertyDependencies{"A": []resource.URN{urnD}}, nil),
			newResource(urnJ, "6", "",
				[]resource.URN{urnD},
				propertyDependencies{"B": []resource.URN{urnD}}, nil),
			newResource(urnK, "7", "",
				[]resource.URN{urnF, urnG},
				propertyDependencies{"A": []resource.URN{urnF, urnG}}, nil),
			newResource(urnL, "8", "",
				[]resource.URN{urnF, urnG},
				propertyDependencies{"B": []resource.URN{urnF, urnG}}, nil),
		},
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		register := func(urn resource.URN, provider string, inputs resource.PropertyMap) resource.ID {
			_, id, _, err := monitor.RegisterResource(urn.Type(), string(urn.Name()), true, deploytest.ResourceOptions{
				Provider: provider,
				Inputs:   inputs,
			})
			assert.NoError(t, err)
			return id
		}

		idA := register(urnA, "", resource.PropertyMap{"A": resource.NewStringProperty("bar")})
		register(urnB, string(urnA)+"::"+string(idA), nil)
		idC := register(urnC, "", nil)
		idD := register(urnD, "", nil)
		register(urnE, string(urnC)+"::"+string(idC), nil)
		register(urnF, "", nil)
		register(urnG, "", nil)
		register(urnH, string(urnD)+"::"+string(idD), nil)
		register(urnI, "", nil)
		register(urnJ, "", nil)
		register(urnK, "", nil)
		register(urnL, "", nil)

		return nil
	})

	return urns, old, program
}

func TestDeleteBeforeReplace(t *testing.T) {
	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L
	//
	// For a given resource R in (A, C, D):
	// - R will be the provider for its first dependent
	// - A change to R will require that its second dependent be replaced
	// - A change to R will not require that its third dependent be replaced
	//
	// In addition, K will have a requires-replacement property that depends on both F and G, and
	// L will have a normal property that depends on both F and G.
	//
	// With that in mind, the following resources should require replacement: A, B, C, E, F, and K

	p := &TestPlan{}

	urns, old, program := generateComplexTestDependencyGraph(t, p)
	names := complexTestDependencyGraphNames

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"A"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		SkipPreview:   true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			replaced := make(map[resource.URN]bool)
			for _, entry := range entries {
				if entry.Step.Op() == deploy.OpReplace {
					replaced[entry.Step.URN()] = true
				}
			}

			assert.Equal(t, map[resource.URN]bool{
				pickURN(t, urns, names, "A"): true,
				pickURN(t, urns, names, "B"): true,
				pickURN(t, urns, names, "C"): true,
				pickURN(t, urns, names, "E"): true,
				pickURN(t, urns, names, "F"): true,
				pickURN(t, urns, names, "K"): true,
			}, replaced)

			return res
		},
	}}

	p.Run(t, old)
}

func TestPropertyDependenciesAdapter(t *testing.T) {
	// Ensure that the eval source properly shims in property dependencies if none were reported (and does not if
	// any were reported).

	type propertyDependencies map[resource.PropertyKey][]resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	const resType = "pkgA:m:typA"
	var urnA, urnB, urnC, urnD resource.URN
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {

		register := func(name string, inputs resource.PropertyMap, inputDeps propertyDependencies,
			dependencies []resource.URN) resource.URN {

			urn, _, _, err := monitor.RegisterResource(resType, name, true, deploytest.ResourceOptions{
				Inputs:       inputs,
				Dependencies: dependencies,
				PropertyDeps: inputDeps,
			})
			assert.NoError(t, err)

			return urn
		}

		urnA = register("A", nil, nil, nil)
		urnB = register("B", nil, nil, nil)
		urnC = register("C", resource.PropertyMap{
			"A": resource.NewStringProperty("foo"),
			"B": resource.NewStringProperty("bar"),
		}, nil, []resource.URN{urnA, urnB})
		urnD = register("D", resource.PropertyMap{
			"A": resource.NewStringProperty("foo"),
			"B": resource.NewStringProperty("bar"),
		}, propertyDependencies{
			"A": []resource.URN{urnB},
			"B": []resource.URN{urnA, urnC},
		}, []resource.URN{urnA, urnB, urnC})

		return nil
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update}},
	}
	snap := p.Run(t, nil)
	for _, res := range snap.Resources {
		switch res.URN {
		case urnA, urnB:
			assert.Empty(t, res.Dependencies)
			assert.Empty(t, res.PropertyDependencies)
		case urnC:
			assert.Equal(t, []resource.URN{urnA, urnB}, res.Dependencies)
			assert.EqualValues(t, propertyDependencies{
				"A": res.Dependencies,
				"B": res.Dependencies,
			}, res.PropertyDependencies)
		case urnD:
			assert.Equal(t, []resource.URN{urnA, urnB, urnC}, res.Dependencies)
			assert.EqualValues(t, propertyDependencies{
				"A": []resource.URN{urnB},
				"B": []resource.URN{urnA, urnC},
			}, res.PropertyDependencies)
		}
	}
}

func TestExplicitDeleteBeforeReplace(t *testing.T) {
	p := &TestPlan{}

	dbrDiff := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: dbrDiff,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	const resType = "pkgA:index:typ"

	inputsA := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})
	dbrValue, dbrA := true, (*bool)(nil)
	inputsB := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})

	var provURN, urnA, urnB resource.URN
	var provID resource.ID
	var err error
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err = monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}
		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)
		provA := provRef.String()

		urnA, _, _, err = monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Provider:            provA,
			Inputs:              inputsA,
			DeleteBeforeReplace: dbrA,
		})
		assert.NoError(t, err)

		inputDepsB := map[resource.PropertyKey][]resource.URN{"A": {urnA}}
		urnB, _, _, err = monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Provider:     provA,
			Inputs:       inputsB,
			Dependencies: []resource.URN{urnA},
			PropertyDeps: inputDepsB,
		})
		assert.NoError(t, err)

		return nil
	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	// Change the value of resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	inputsA["A"] = resource.NewStringProperty("bar")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it requires delete-before-replace and change the value of resA.A. Both
	// resA and resB should be replaced, and the replacements should be delete-before-replace.
	dbrA, inputsA["A"] = &dbrValue, resource.NewStringProperty("baz")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpCreateReplacement, URN: urnB},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the value of resB.A. Only resB should be replaced, and the replacement should be create-before-delete.
	inputsB["A"] = resource.NewStringProperty("qux")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpSame, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnB},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it no longer requires delete-before-replace and change the value of
	// resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	dbrA, inputsA["A"] = nil, resource.NewStringProperty("zam")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the diff of resA such that it requires delete-before-replace and change the value of resA.A. Both
	// resA and resB should be replaced, and the replacements should be delete-before-replace.
	dbrDiff, inputsA["A"] = true, resource.NewStringProperty("foo")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpCreateReplacement, URN: urnB},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it disables delete-before-replace and change the value of
	// resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	dbrA, dbrValue, inputsA["A"] = &dbrValue, false, resource.NewStringProperty("bar")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	p.Run(t, snap)
}

func TestSingleResourceIgnoreChanges(t *testing.T) {
	var expectedIgnoreChanges []string

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return resource.PropertyMap{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	updateProgramWithProps := func(snap *deploy.Snapshot, props resource.PropertyMap, ignoreChanges []string,
		allowedOps []deploy.StepOp) *deploy.Snapshot {
		expectedIgnoreChanges = ignoreChanges
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:        props,
				IgnoreChanges: ignoreChanges,
			})
			assert.NoError(t, err)
			return nil
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []deploy.StepOp{payload.Metadata.Op})
							}
						}
						return res
					},
				},
			},
		}
		return p.Run(t, snap)
	}

	snap := updateProgramWithProps(nil, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": "foo",
		},
	}), []string{"a", "b.c"}, []deploy.StepOp{deploy.OpCreate})

	// Ensure that a change to an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 2,
		"b": map[string]interface{}{
			"c": "bar",
		},
	}), []string{"a", "b.c"}, []deploy.StepOp{deploy.OpSame})

	// Ensure that a change to an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": map[string]interface{}{
			"c": "qux",
		},
	}), nil, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that a removing an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.PropertyMap{}, []string{"a", "b"}, []deploy.StepOp{deploy.OpSame})

	// Ensure that a removing an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.PropertyMap{}, nil, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that adding an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 4,
		"b": map[string]interface{}{
			"c": "zed",
		},
	}), []string{"a", "b"}, []deploy.StepOp{deploy.OpSame})

	// Ensure that adding an un-ignored property results in an OpUpdate
	_ = updateProgramWithProps(snap, resource.PropertyMap{
		"c": resource.NewNumberProperty(4),
	}, []string{"a", "b"}, []deploy.StepOp{deploy.OpUpdate})
}

// TestDefaultProviderDiff tests that the engine can gracefully recover whenever a resource's default provider changes
// and there is no diff in the provider's inputs.
func TestDefaultProviderDiff(t *testing.T) {
	const resName, resBName = "resA", "resB"
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.10"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.11"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.12"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	runProgram := func(base *deploy.Snapshot, versionA, versionB string, expectedStep deploy.StepOp) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", resName, true, deploytest.ResourceOptions{
				Version: versionA,
			})
			assert.NoError(t, err)
			_, _, _, err = monitor.RegisterResource("pkgA:m:typA", resBName, true, deploytest.ResourceOptions{
				Version: versionB,
			})
			assert.NoError(t, err)
			return nil
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, entry := range entries {
							if entry.Kind != JournalEntrySuccess {
								continue
							}

							switch entry.Step.URN().Name().String() {
							case resName, resBName:
								assert.Equal(t, expectedStep, entry.Step.Op())
							}
						}
						return res
					},
				},
			},
		}
		return p.Run(t, base)
	}

	// This test simulates the upgrade scenario of old-style default providers to new-style versioned default providers.
	//
	// The first update creates a stack using a language host that does not report a version to the engine. As a result,
	// the engine makes up a default provider for "pkgA" and calls it "default". It then creates the two resources that
	// we are creating and associates them with the default provider.
	snap := runProgram(nil, "", "", deploy.OpCreate)
	for _, res := range snap.Resources {
		switch {
		case providers.IsDefaultProvider(res.URN):
			assert.Equal(t, "default", res.URN.Name().String())
		case res.URN.Name().String() == resName || res.URN.Name().String() == resBName:
			provRef, err := providers.ParseReference(res.Provider)
			assert.NoError(t, err)
			assert.Equal(t, "default", provRef.URN().Name().String())
		}
	}

	// The second update switches to a language host that does report a version to the engine. As a result, the engine
	// uses this version to make a new provider, with a different URN, and uses that provider to operate on resA and
	// resB.
	//
	// Despite switching out the provider, the engine should still generate a Same step for resA. It is vital that the
	// engine gracefully react to changes in the default provider in this manner. See pulumi/pulumi#2753 for what
	// happens when it doesn't.
	snap = runProgram(snap, "0.17.10", "0.17.10", deploy.OpSame)
	for _, res := range snap.Resources {
		switch {
		case providers.IsDefaultProvider(res.URN):
			assert.Equal(t, "default_0_17_10", res.URN.Name().String())
		case res.URN.Name().String() == resName || res.URN.Name().String() == resBName:
			provRef, err := providers.ParseReference(res.Provider)
			assert.NoError(t, err)
			assert.Equal(t, "default_0_17_10", provRef.URN().Name().String())
		}
	}

	// The third update changes the version that the language host reports to the engine. This simulates a scenario in
	// which a user updates their SDK to a new version of a provider package. In order to simulate side-by-side
	// packages with different versions, this update requests distinct package versions for resA and resB.
	snap = runProgram(snap, "0.17.11", "0.17.12", deploy.OpSame)
	for _, res := range snap.Resources {
		switch {
		case providers.IsDefaultProvider(res.URN):
			assert.True(t, res.URN.Name().String() == "default_0_17_11" || res.URN.Name().String() == "default_0_17_12")
		case res.URN.Name().String() == resName:
			provRef, err := providers.ParseReference(res.Provider)
			assert.NoError(t, err)
			assert.Equal(t, "default_0_17_11", provRef.URN().Name().String())
		case res.URN.Name().String() == resBName:
			provRef, err := providers.ParseReference(res.Provider)
			assert.NoError(t, err)
			assert.Equal(t, "default_0_17_12", provRef.URN().Name().String())
		}
	}
}

// TestDefaultProviderDiffReplacement tests that, when replacing a default provider for a resource, the engine will
// replace the resource if DiffConfig on the new provider returns a diff for the provider's new state.
func TestDefaultProviderDiffReplacement(t *testing.T) {
	const resName, resBName = "resA", "resB"
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.10"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// This implementation of DiffConfig always requests replacement.
				DiffConfigF: func(_ resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					keys := []resource.PropertyKey{}
					for k := range news {
						keys = append(keys, k)
					}
					return plugin.DiffResult{
						Changes:     plugin.DiffSome,
						ReplaceKeys: keys,
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.11"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	runProgram := func(base *deploy.Snapshot, versionA, versionB string, expectedSteps ...deploy.StepOp) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", resName, true, deploytest.ResourceOptions{
				Version: versionA,
			})
			assert.NoError(t, err)
			_, _, _, err = monitor.RegisterResource("pkgA:m:typA", resBName, true, deploytest.ResourceOptions{
				Version: versionB,
			})
			assert.NoError(t, err)
			return nil
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, entry := range entries {
							if entry.Kind != JournalEntrySuccess {
								continue
							}

							switch entry.Step.URN().Name().String() {
							case resName:
								assert.Subset(t, expectedSteps, []deploy.StepOp{entry.Step.Op()})
							case resBName:
								assert.Subset(t,
									[]deploy.StepOp{deploy.OpCreate, deploy.OpSame}, []deploy.StepOp{entry.Step.Op()})
							}
						}
						return res
					},
				},
			},
		}
		return p.Run(t, base)
	}

	// This test simulates the upgrade scenario of default providers, except that the requested upgrade results in the
	// provider getting replaced. Because of this, the engine should decide to replace resA. It should not decide to
	// replace resB, as its change does not require replacement.
	snap := runProgram(nil, "", "", deploy.OpCreate)
	for _, res := range snap.Resources {
		switch {
		case providers.IsDefaultProvider(res.URN):
			assert.Equal(t, "default", res.URN.Name().String())
		case res.URN.Name().String() == resName || res.URN.Name().String() == resBName:
			provRef, err := providers.ParseReference(res.Provider)
			assert.NoError(t, err)
			assert.Equal(t, "default", provRef.URN().Name().String())
		}
	}

	// Upon update, now that the language host is sending a version, DiffConfig reports that there's a diff between the
	// old and new provider and so we must replace resA.
	snap = runProgram(snap, "0.17.10", "0.17.11", deploy.OpCreateReplacement, deploy.OpReplace, deploy.OpDeleteReplaced)
	for _, res := range snap.Resources {
		switch {
		case providers.IsDefaultProvider(res.URN):
			assert.True(t, res.URN.Name().String() == "default_0_17_10" || res.URN.Name().String() == "default_0_17_11")
		case res.URN.Name().String() == resName:
			provRef, err := providers.ParseReference(res.Provider)
			assert.NoError(t, err)
			assert.Equal(t, "default_0_17_10", provRef.URN().Name().String())
		case res.URN.Name().String() == resBName:
			provRef, err := providers.ParseReference(res.Provider)
			assert.NoError(t, err)
			assert.Equal(t, "default_0_17_11", provRef.URN().Name().String())
		}
	}
}

// Resource is an abstract representation of a resource graph
type Resource struct {
	t                   tokens.Type
	name                string
	children            []Resource
	props               resource.PropertyMap
	aliases             []resource.URN
	dependencies        []resource.URN
	parent              resource.URN
	deleteBeforeReplace bool
}

func registerResources(t *testing.T, monitor *deploytest.ResourceMonitor, resources []Resource) error {
	for _, r := range resources {
		_, _, _, err := monitor.RegisterResource(r.t, r.name, true, deploytest.ResourceOptions{
			Parent:              r.parent,
			Dependencies:        r.dependencies,
			Inputs:              r.props,
			DeleteBeforeReplace: &r.deleteBeforeReplace,
			Aliases:             r.aliases,
		})
		if err != nil {
			return err
		}
		err = registerResources(t, monitor, r.children)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestAliases(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(res resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					replaceKeys := []resource.PropertyKey{}
					old, hasOld := olds["forcesReplacement"]
					new, hasNew := news["forcesReplacement"]
					if hasOld && !hasNew || hasNew && !hasOld || hasOld && hasNew && old.Diff(new) != nil {
						replaceKeys = append(replaceKeys, "forcesReplacement")
					}
					return plugin.DiffResult{ReplaceKeys: replaceKeys}, nil
				},
			}, nil
		}),
	}

	updateProgramWithResource := func(
		snap *deploy.Snapshot, resources []Resource, allowedOps []deploy.StepOp) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			err := registerResources(t, monitor, resources)
			return err
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []deploy.StepOp{payload.Metadata.Op})
							}
						}

						for _, entry := range entries {
							if entry.Step.Type() == "pulumi:providers:pkgA" {
								continue
							}
							switch entry.Kind {
							case JournalEntrySuccess:
								assert.Subset(t, allowedOps, []deploy.StepOp{entry.Step.Op()})
							case JournalEntryFailure:
								assert.Fail(t, "unexpected failure in journal")
							case JournalEntryBegin:
							case JournalEntryOutputs:
							}
						}

						return res
					},
				},
			},
		}
		return p.Run(t, snap)
	}

	snap := updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []deploy.StepOp{deploy.OpCreate})

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:       "pkgA:index:t1",
		name:    "n2",
		aliases: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n3",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that changing everything (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t4",
		name: "n2",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(42),
		},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
		},
	}}, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that changing everything again (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t5",
		name: "n3",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t4::n2",
		},
	}}, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that changing a forceNew property while also changing type and name leads to replacement not delete+create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t6",
		name: "n4",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t5::n3",
		},
	}}, []deploy.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// Ensure that changing a forceNew property and deleteBeforeReplace while also changing type and name leads to
	// replacement not delete+create
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t7",
		name: "n5",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(999),
		},
		deleteBeforeReplace: true,
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t6::n4",
		},
	}}, []deploy.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// Start again - this time with two resources with depends on relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:            "pkgA:index:t2",
		name:         "n2",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []deploy.StepOp{deploy.OpCreate})

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:            "pkgA:index:t2-new",
		name:         "n2-new",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1-new::n1-new"},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t2::n2",
		},
	}}, []deploy.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// Start again - this time with two resources with parent relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:      "pkgA:index:t2",
		name:   "n2",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}}, []deploy.StepOp{deploy.OpCreate})

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:      "pkgA:index:t2-new",
		name:   "n2-new",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n2",
		},
	}}, []deploy.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// ensure failure when different resources use duplicate aliases
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpCreate})

	err := snap.NormalizeURNReferences()
	assert.Equal(t, err.Error(),
		"Two resources ('urn:pulumi:test::test::pkgA:index:t1::n1'"+
			" and 'urn:pulumi:test::test::pkgA:index:t2::n2') aliased to the same: 'urn:pulumi:test::test::pkgA:index:t1::n1'")

	// ensure different resources can use different aliases
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []deploy.StepOp{deploy.OpCreate})

	err = snap.NormalizeURNReferences()
	assert.Nil(t, err)
}

func TestPersistentDiff(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

			found := false
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					p := e.Payload().(ResourcePreEventPayload).Metadata
					if p.URN == resURN {
						assert.Equal(t, deploy.OpUpdate, p.Op)
						found = true
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)

	// Next, enable legacy diff behavior. We should see no changes to the resource.
	p.Options.UseLegacyDiff = true
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

			found := false
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					p := e.Payload().(ResourcePreEventPayload).Metadata
					if p.URN == resURN {
						assert.Equal(t, deploy.OpSame, p.Op)
						found = true
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)
}

func TestDetailedDiffReplace(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					return plugin.DiffResult{
						Changes: plugin.DiffSome,
						DetailedDiff: map[string]plugin.PropertyDiff{
							"prop": {Kind: plugin.DiffAddReplace},
						},
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

			found := false
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					p := e.Payload().(ResourcePreEventPayload).Metadata
					if p.URN == resURN && p.Op == deploy.OpReplace {
						found = true
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)
}

func TestImportOption(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if olds["foo"].DeepEquals(news["foo"]) {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}

					diffKind := plugin.DiffUpdate
					if news["foo"].IsString() && news["foo"].StringValue() == "replace" {
						diffKind = plugin.DiffUpdateReplace
					}

					return plugin.DiffResult{
						Changes: plugin.DiffSome,
						DetailedDiff: map[string]plugin.PropertyDiff{
							"foo": {Kind: diffKind},
						},
					}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo": resource.NewStringProperty("bar"),
						},
						Outputs: resource.PropertyMap{
							"foo": resource.NewStringProperty("bar"),
						},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	readID, importID, inputs := resource.ID(""), resource.ID("id"), resource.PropertyMap{}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		if readID != "" {
			_, _, err = monitor.ReadResource("pkgA:m:typA", "resA", readID, "", resource.PropertyMap{}, "", "")
		} else {
			_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:   inputs,
				ImportID: importID,
			})
		}
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update. The import should fail due to a mismatch in inputs between the program and the
	// actual resource state.
	project := p.GetProject()
	_, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)

	// Run a second update after fixing the inputs. The import should succeed.
	inputs["foo"] = resource.NewStringProperty("bar")
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN:
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				case resURN:
					assert.Equal(t, deploy.OpImport, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 2)

	// Now, run another update. The update should succeed and there should be no diffs.
	snap, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)

	// Change a property value and run a third update. The update should succeed.
	inputs["foo"] = resource.NewStringProperty("rab")
	snap, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				case resURN:
					assert.Equal(t, deploy.OpUpdate, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)

	// Change the property value s.t. the resource requires replacement. The update should fail.
	inputs["foo"] = resource.NewStringProperty("replace")
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)

	// Finally, destroy the stack. The `Delete` function should be called.
	_, res = TestOp(Destroy).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)

	// Now clear the ID to import and run an initial update to create a resource that we will import-replace.
	importID, inputs["foo"] = "", resource.NewStringProperty("bar")
	snap, res = TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 2)

	// Set the import ID to the same ID as the existing resource and run an update. This should produce no changes.
	for _, r := range snap.Resources {
		if r.URN == resURN {
			importID = r.ID
		}
	}
	snap, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)

	// Then set the import ID and run another update. The update should succeed and should show an import-replace and
	// a delete-replaced.
	importID = "id"
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				case resURN:
					switch entry.Step.Op() {
					case deploy.OpReplace, deploy.OpImportReplacement:
						assert.Equal(t, importID, entry.Step.New().ID)
					case deploy.OpDeleteReplaced:
						assert.NotEqual(t, importID, entry.Step.Old().ID)
					}
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)

	// Change the program to read a resource rather than creating one.
	readID = "id"
	snap, res = TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN:
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				case resURN:
					assert.Equal(t, deploy.OpRead, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 2)

	// Now have the program import the resource. We should see an import-replace and a read-discard.
	readID, importID = "", readID
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				case resURN:
					switch entry.Step.Op() {
					case deploy.OpReplace, deploy.OpImportReplacement:
						assert.Equal(t, importID, entry.Step.New().ID)
					case deploy.OpDiscardReplaced:
						assert.Equal(t, importID, entry.Step.Old().ID)
					}
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)
}

// TestImportWithDifferingImportIdentifierFormat tests importing a resource that has a different format of identifier
// for the import input than for the ID property, ensuring that a second update does not result in a replace.
func TestImportWithDifferingImportIdentifierFormat(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if olds["foo"].DeepEquals(news["foo"]) {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}

					return plugin.DiffResult{
						Changes: plugin.DiffSome,
						DetailedDiff: map[string]plugin.PropertyDiff{
							"foo": {Kind: plugin.DiffUpdate},
						},
					}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						// This ID is deliberately not the same as the ID used to import.
						ID: "id",
						Inputs: resource.PropertyMap{
							"foo": resource.NewStringProperty("bar"),
						},
						Outputs: resource.PropertyMap{
							"foo": resource.NewStringProperty("bar"),
						},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
			// The import ID is deliberately not the same as the ID returned from Read.
			ImportID: resource.ID("import-id"),
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update. The import should succeed.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN:
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				case resURN:
					assert.Equal(t, deploy.OpImport, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 2)

	// Now, run another update. The update should succeed and there should be no diffs.
	snap, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, res result.Result) result.Result {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return res
		})
	assert.Nil(t, res)
}

func TestCustomTimeouts(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			CustomTimeouts: &resource.CustomTimeouts{
				Create: 60, Delete: 60, Update: 240,
			},
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default")
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.NotNil(t, snap.Resources[1].CustomTimeouts)
	assert.Equal(t, snap.Resources[1].CustomTimeouts.Create, float64(60))
	assert.Equal(t, snap.Resources[1].CustomTimeouts.Update, float64(240))
	assert.Equal(t, snap.Resources[1].CustomTimeouts.Delete, float64(60))
}

func TestProviderDiffMissingOldOutputs(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					// Always require replacement if any diff exists.
					if !olds.DeepEquals(news) {
						keys := []resource.PropertyKey{}
						for k := range news {
							keys = append(keys, k)
						}
						return plugin.DiffResult{Changes: plugin.DiffSome, ReplaceKeys: keys}, nil
					}
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Config: config.Map{
			config.MustMakeKey("pkgA", "foo"): config.NewValue("bar"),
		},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its initial update and refresh.
	p.Steps = steps[:2]
	snap := p.Run(t, nil)

	// Delete the old provider outputs (if any) from the checkpoint, then run the no-op update.
	providerURN := p.NewProviderURN("pkgA", "default", "")
	for _, r := range snap.Resources {
		if r.URN == providerURN {
			r.Outputs = nil
		}
	}

	p.Steps = steps[2:3]
	snap = p.Run(t, snap)

	// Change the config, delete the old provider outputs,  and run an update. We expect everything to require
	// replacement.
	p.Config[config.MustMakeKey("pkgA", "foo")] = config.NewValue("baz")
	for _, r := range snap.Resources {
		if r.URN == providerURN {
			r.Outputs = nil
		}
	}
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			replacedProvider, replacedResource := false, false
			for _, entry := range entries {
				if entry.Kind != JournalEntrySuccess || entry.Step.Op() != deploy.OpDeleteReplaced {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case providerURN:
					replacedProvider = true
				case resURN:
					replacedResource = true
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, replacedProvider)
			assert.True(t, replacedResource)

			return res
		},
	}}
	p.Run(t, snap)
}

func TestRefreshStepWillPersistUpdatedIDs(t *testing.T) {
	p := &TestPlan{}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	idBefore := resource.ID("myid")
	idAfter := resource.ID("mynewid")
	outputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{ID: idAfter, Outputs: outputs, Inputs: resource.PropertyMap{}}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", false)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Options.Host = host

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:       resURN.Type(),
				URN:        resURN,
				Custom:     true,
				ID:         idBefore,
				Inputs:     resource.PropertyMap{},
				Outputs:    outputs,
				InitErrors: []string{"Resource failed to initialize"},
			},
		},
	}

	p.Steps = []TestStep{{Op: Refresh, SkipPreview: true}}
	snap := p.Run(t, old)

	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Empty(t, resource.InitErrors)
			assert.Equal(t, idAfter, resource.ID)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}
}

func TestMissingRead(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ resource.URN, _ resource.ID, _, _ resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	// Our program reads a resource and exits.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "", "")
		assert.Error(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update, ExpectFailure: true}},
	}
	p.Run(t, nil)
}

func TestImportUpdatedID(t *testing.T) {
	p := &TestPlan{}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	importID := resource.ID("myID")
	actualID := resource.ID("myNewID")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{
						ID:      actualID,
						Outputs: resource.PropertyMap{},
						Inputs:  resource.PropertyMap{},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, id, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			ImportID: importID,
		})
		assert.NoError(t, err)
		assert.Equal(t, actualID, id)
		return nil
	})
	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Steps = []TestStep{{Op: Refresh, SkipPreview: true}}
	snap := p.Run(t, nil)

	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Equal(t, actualID, resource.ID)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}
}

func TestDestroyTarget(t *testing.T) {
	// Try refreshing a stack with combinations of the above resources as target to destroy.
	subsets := combinations.All(complexTestDependencyGraphNames)

	for _, subset := range subsets {
		// limit to up to 3 resources to destroy.  This keeps the test running time under
		// control as it only generates a few hundred combinations instead of several thousand.
		if len(subset) <= 3 {
			destroySpecificTargets(t, subset, true, /*targetDependents*/
				func(urns []resource.URN, deleted map[resource.URN]bool) {})
		}
	}

	destroySpecificTargets(
		t, []string{"A"}, true, /*targetDependents*/
		func(urns []resource.URN, deleted map[resource.URN]bool) {
			// when deleting 'A' we expect A, B, C, E, F, and K to be deleted
			names := complexTestDependencyGraphNames
			assert.Equal(t, map[resource.URN]bool{
				pickURN(t, urns, names, "A"): true,
				pickURN(t, urns, names, "B"): true,
				pickURN(t, urns, names, "C"): true,
				pickURN(t, urns, names, "E"): true,
				pickURN(t, urns, names, "F"): true,
				pickURN(t, urns, names, "K"): true,
			}, deleted)
		})

	destroySpecificTargets(
		t, []string{"A"}, false, /*targetDependents*/
		func(urns []resource.URN, deleted map[resource.URN]bool) {})
}

func destroySpecificTargets(
	t *testing.T, targets []string, targetDependents bool,
	validate func(urns []resource.URN, deleted map[resource.URN]bool)) {

	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L

	p := &TestPlan{}

	urns, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"A"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Options.TargetDependents = targetDependents

	destroyTargets := []resource.URN{}
	for _, target := range targets {
		destroyTargets = append(destroyTargets, pickURN(t, urns, complexTestDependencyGraphNames, target))
	}

	p.Options.DestroyTargets = destroyTargets
	t.Logf("Destroying targets: %v", destroyTargets)

	// If we're not forcing the targets to be destroyed, then expect to get a failure here as
	// we'll have downstream resources to delete that weren't specified explicitly.
	p.Steps = []TestStep{{
		Op:            Destroy,
		ExpectFailure: !targetDependents,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			deleted := make(map[resource.URN]bool)
			for _, entry := range entries {
				assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				deleted[entry.Step.URN()] = true
			}

			for _, target := range p.Options.DestroyTargets {
				assert.Contains(t, deleted, target)
			}

			validate(urns, deleted)
			return res
		},
	}}

	p.Run(t, old)
}

func TestUpdateTarget(t *testing.T) {
	// Try refreshing a stack with combinations of the above resources as target to destroy.
	subsets := combinations.All(complexTestDependencyGraphNames)

	for _, subset := range subsets {
		// limit to up to 3 resources to destroy.  This keeps the test running time under
		// control as it only generates a few hundred combinations instead of several thousand.
		if len(subset) <= 3 {
			updateSpecificTargets(t, subset)
		}
	}

	updateSpecificTargets(t, []string{"A"})

	// Also update a target that doesn't exist to make sure we don't crash or otherwise go off the rails.
	updateInvalidTarget(t)
}

func updateSpecificTargets(t *testing.T, targets []string) {
	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L

	p := &TestPlan{}

	urns, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					outputs := olds.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return outputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	updateTargets := []resource.URN{}
	for _, target := range targets {
		updateTargets = append(updateTargets,
			pickURN(t, urns, complexTestDependencyGraphNames, target))
	}

	p.Options.UpdateTargets = updateTargets
	t.Logf("Updating targets: %v", updateTargets)

	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			updated := make(map[resource.URN]bool)
			sames := make(map[resource.URN]bool)
			for _, entry := range entries {
				if entry.Step.Op() == deploy.OpUpdate {
					updated[entry.Step.URN()] = true
				} else if entry.Step.Op() == deploy.OpSame {
					sames[entry.Step.URN()] = true
				} else {
					assert.FailNowf(t, "", "Got a step that wasn't a same/update: %v", entry.Step.Op())
				}
			}

			for _, target := range p.Options.UpdateTargets {
				assert.Contains(t, updated, target)
			}

			for _, target := range p.Options.UpdateTargets {
				assert.NotContains(t, sames, target)
			}

			return res
		},
	}}
	p.Run(t, old)
}

func updateInvalidTarget(t *testing.T) {
	p := &TestPlan{}

	_, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					outputs := olds.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return outputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Options.UpdateTargets = []resource.URN{"foo"}
	t.Logf("Updating invalid targets: %v", p.Options.UpdateTargets)

	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
	}}

	p.Run(t, old)
}

func TestCreateDuringTargetedUpdate_CreateMentionedAsTarget(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		return nil
	})
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")
	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA, resB}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				if entry.Step.URN() == resA {
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				} else if entry.Step.URN() == resB {
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				}
			}

			return res
		},
	}}
	p.Run(t, snap1)
}

func TestCreateDuringTargetedUpdate_UntargetedCreateNotReferenced(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		return nil
	})
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	resA := p.NewURN("pkgA:m:typA", "resA", "")

	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				// everything should be a same op here.
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}

			return res
		},
	}}
	p.Run(t, snap1)
}

func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByTarget(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	p.Run(t, nil)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	// Now, create a resource resB.  But reference it from A. This will cause a dependency we can't
	// satisfy.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true,
			deploytest.ResourceOptions{
				Dependencies: []resource.URN{resB},
			})
		assert.NoError(t, err)

		return nil
	})
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
	}}
	p.Run(t, nil)
}

func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByUntargetedCreate(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	// Now, create a resource resB.  But reference it from A. This will cause a dependency we can't
	// satisfy.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true,
			deploytest.ResourceOptions{
				Dependencies: []resource.URN{resB},
			})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		return nil
	})
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}

			return res
		},
	}}
	p.Run(t, snap1)
}

func TestDependencyChangeDBR(t *testing.T) {
	p := &TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					if !olds["B"].DeepEquals(news["B"]) {
						return plugin.DiffResult{
							Changes: plugin.DiffSome,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	const resType = "pkgA:index:typ"

	inputsA := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})
	inputsB := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})

	var urnA, urnB resource.URN
	var err error
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urnA, _, _, err = monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})
		assert.NoError(t, err)

		inputDepsB := map[resource.PropertyKey][]resource.URN{"A": {urnA}}
		urnB, _, _, err = monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Inputs:       inputsB,
			Dependencies: []resource.URN{urnA},
			PropertyDeps: inputDepsB,
		})
		assert.NoError(t, err)

		return nil
	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	inputsA["A"] = resource.NewStringProperty("bar")
	program = deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urnB, _, _, err = monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Inputs: inputsB,
		})
		assert.NoError(t, err)

		urnA, _, _, err = monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})
		assert.NoError(t, err)

		return nil
	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result) result.Result {

				assert.Nil(t, res)
				assert.True(t, len(entries) > 0)

				resBDeleted, resBSame := false, false
				for _, entry := range entries {
					if entry.Step.URN() == urnB {
						switch entry.Step.Op() {
						case deploy.OpDelete, deploy.OpDeleteReplaced:
							resBDeleted = true
						case deploy.OpSame:
							resBSame = true
						}
					}
				}
				assert.True(t, resBSame)
				assert.False(t, resBDeleted)

				return res
			},
		},
	}
	p.Run(t, snap)
}

func TestReplaceSpecificTargets(t *testing.T) {
	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L

	p := &TestPlan{}

	urns, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					// No resources will change.
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},

				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	getURN := func(name string) resource.URN {
		return pickURN(t, urns, complexTestDependencyGraphNames, name)
	}

	p.Options.ReplaceTargets = []resource.URN{
		getURN("F"),
		getURN("B"),
		getURN("G"),
	}

	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			replaced := make(map[resource.URN]bool)
			sames := make(map[resource.URN]bool)
			for _, entry := range entries {
				if entry.Step.Op() == deploy.OpReplace {
					replaced[entry.Step.URN()] = true
				} else if entry.Step.Op() == deploy.OpSame {
					sames[entry.Step.URN()] = true
				}
			}

			for _, target := range p.Options.ReplaceTargets {
				assert.Contains(t, replaced, target)
			}

			for _, target := range p.Options.ReplaceTargets {
				assert.NotContains(t, sames, target)
			}

			return res
		},
	}}

	p.Run(t, old)
}

func TestProviderPreview(t *testing.T) {
	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return "created-id", news, resource.StatusOK, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	preview := true
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
		if !preview {
			computed = "alpha"
		}

		ins := resource.NewPropertyMapFromMap(map[string]interface{}{
			"foo": "bar",
			"baz": map[string]interface{}{
				"a": 42,
				"b": computed,
			},
			"qux": []interface{}{
				computed,
				24,
			},
			"zed": computed,
		})

		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		assert.True(t, state.DeepEquals(ins))

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run a preview. The inputs should be propagated to the outputs by the provider during the create.
	preview, sawPreview = true, false
	_, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should be propagated to the outputs during the update.
	preview, sawPreview = true, false
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)
}

type testResource struct {
	pulumi.CustomResourceState

	Foo pulumi.StringOutput `pulumi:"foo"`
}

type testResourceArgs struct {
	Foo  string `pulumi:"foo"`
	Bar  string `pulumi:"bar"`
	Baz  string `pulumi:"baz"`
	Bang string `pulumi:"bang"`
}

type testResourceInputs struct {
	Foo  pulumi.StringInput
	Bar  pulumi.StringInput
	Baz  pulumi.StringInput
	Bang pulumi.StringInput
}

func (*testResourceInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testResourceArgs)(nil))
}

func TestSingleResourceDefaultProviderGolangLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			var resA testResource
			err := ctx.RegisterResource("pkgA:m:typA", "resA", &testResourceInputs{
				Foo: pulumi.String("bar"),
			}, &resA)
			assert.NoError(t, err)

			var resB testResource
			err = ctx.RegisterResource("pkgA:m:typA", "resB", &testResourceInputs{
				Baz: resA.Foo.ApplyT(func(v string) string {
					return v + "bar"
				}).(pulumi.StringOutput),
			}, &resB)
			assert.NoError(t, err)

			return nil
		})
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// Inspired by transformations_test.go.
func TestSingleResourceDefaultProviderGolangTransformations(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	newResource := func(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) error {
		var res testResource
		return ctx.RegisterResource("pkgA:m:typA", name, &testResourceInputs{
			Foo: pulumi.String("bar"),
		}, &res, opts...)
	}

	newComponent := func(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) error {
		var res testResource
		err := ctx.RegisterComponentResource("pkgA:m:typA", name, &res, opts...)
		if err != nil {
			return err
		}

		var resChild testResource
		return ctx.RegisterResource("pkgA:m:typA", name+"Child", &testResourceInputs{
			Foo: pulumi.String("bar"),
		}, &resChild, pulumi.Parent(&res))
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			// Scenario #1 - apply a transformation to a CustomResource
			res1Transformation := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				// TODO[pulumi/pulumi#3846] We should use a mergeOptions-style API here.
				return &pulumi.ResourceTransformationResult{
					Props: args.Props,
					Opts:  append(args.Opts, pulumi.AdditionalSecretOutputs([]string{"output"})),
				}
			}
			assert.NoError(t, newResource(ctx, "res1",
				pulumi.Transformations([]pulumi.ResourceTransformation{res1Transformation})))

			// Scenario #2 - apply a transformation to a Component to transform its children
			res2Transformation := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				if args.Name == "res2Child" {
					// TODO[pulumi/pulumi#3846] We should use a mergeOptions-style API here.
					return &pulumi.ResourceTransformationResult{
						Props: args.Props,
						Opts:  append(args.Opts, pulumi.AdditionalSecretOutputs([]string{"output", "output2"})),
					}
				}

				return nil
			}
			assert.NoError(t, newComponent(ctx, "res2",
				pulumi.Transformations([]pulumi.ResourceTransformation{res2Transformation})))

			// Scenario #3 - apply a transformation to the Stack to transform all (future) resources in the stack
			res3Transformation := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				// Props might be nil.
				var props *testResourceInputs
				if args.Props == nil {
					props = &testResourceInputs{}
				} else {
					props = args.Props.(*testResourceInputs)
				}
				props.Foo = pulumi.String("baz")

				return &pulumi.ResourceTransformationResult{
					Props: props,
					Opts:  args.Opts,
				}
			}
			assert.NoError(t, ctx.RegisterStackTransformation(res3Transformation))
			assert.NoError(t, newResource(ctx, "res3"))

			// Scenario #4 - transformations are applied in order of decreasing specificity
			// 1. (not in this example) Child transformation
			// 2. First parent transformation
			// 3. Second parent transformation
			// 4. Stack transformation
			res4Transformation1 := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				if args.Name == "res4Child" {
					props := args.Props.(*testResourceInputs)
					props.Foo = pulumi.String("baz1")

					return &pulumi.ResourceTransformationResult{
						Props: props,
						Opts:  args.Opts,
					}
				}
				return nil
			}
			res4Transformation2 := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				if args.Name == "res4Child" {
					props := args.Props.(*testResourceInputs)
					props.Foo = pulumi.String("baz2")

					return &pulumi.ResourceTransformationResult{
						Props: props,
						Opts:  args.Opts,
					}
				}
				return nil
			}
			assert.NoError(t, newComponent(ctx, "res4",
				pulumi.Transformations([]pulumi.ResourceTransformation{res4Transformation1, res4Transformation2})))

			return nil
		})
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			foundRes1 := false
			foundRes2 := false
			foundRes2Child := false
			foundRes3 := false
			foundRes4Child := false
			// foundRes5Child1 := false
			for _, res := range entries.Snap(target.Snapshot).Resources {
				// "res1" has a transformation which adds additionalSecretOutputs
				if res.URN.Name() == "res1" {
					foundRes1 = true
					assert.Equal(t, res.Type, tokens.Type("pkgA:m:typA"))
					assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
				}
				// "res2" has a transformation which adds additionalSecretOutputs to it's "child"
				if res.URN.Name() == "res2" {
					foundRes2 = true
					assert.Equal(t, res.Type, tokens.Type("pkgA:m:typA"))
					assert.NotContains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
				}
				if res.URN.Name() == "res2Child" {
					foundRes2Child = true
					assert.Equal(t, res.Parent.Name(), tokens.QName("res2"))
					assert.Equal(t, res.Type, tokens.Type("pkgA:m:typA"))
					assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
					assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output2"))
				}
				// "res3" is impacted by a global stack transformation which sets
				// Foo to "baz"
				if res.URN.Name() == "res3" {
					foundRes3 = true
					assert.Equal(t, "baz", res.Inputs["foo"].StringValue())
					assert.Len(t, res.Aliases, 0)
				}
				// "res4" is impacted by two component parent transformations which set
				// Foo to "baz1" and then "baz2" and also a global stack
				// transformation which sets optionalDefault to "baz".  The end
				// result should be "baz".
				if res.URN.Name() == "res4Child" {
					foundRes4Child = true
					assert.Equal(t, res.Parent.Name(), tokens.QName("res4"))
					assert.Equal(t, "baz", res.Inputs["foo"].StringValue())
				}
			}

			assert.True(t, foundRes1)
			assert.True(t, foundRes2)
			assert.True(t, foundRes2Child)
			assert.True(t, foundRes3)
			assert.True(t, foundRes4Child)
			return res
		},
	}}

	p.Run(t, nil)
}

// This test validates the wiring of the IgnoreChanges prop in the go SDK.
// It doesn't attempt to validate underlying behavior.
func TestIgnoreChangesGolangLifecycle(t *testing.T) {
	var expectedIgnoreChanges []string

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {
					// just verify that the IgnoreChanges prop made it through
					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	setupAndRunProgram := func(ignoreChanges []string) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
				Project:     info.Project,
				Stack:       info.Stack,
				Parallel:    info.Parallel,
				DryRun:      info.DryRun,
				MonitorAddr: info.MonitorAddress,
			})
			assert.NoError(t, err)

			return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
				var res pulumi.CustomResourceState
				err := ctx.RegisterResource("pkgA:m:typA", "resA", nil, &res, pulumi.IgnoreChanges(ignoreChanges))
				assert.NoError(t, err)

				return nil
			})
		})

		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Equal(t, []deploy.StepOp{deploy.OpCreate}, []deploy.StepOp{payload.Metadata.Op})
							}
						}
						return res
					},
				},
			},
		}
		return p.Run(t, nil)
	}

	// ignore changes specified
	ignoreChanges := []string{"b"}
	setupAndRunProgram(ignoreChanges)

	// ignore changes empty
	ignoreChanges = []string{}
	setupAndRunProgram(ignoreChanges)
}

func TestExplicitDeleteBeforeReplaceGoSDK(t *testing.T) {
	p := &TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					if !olds["foo"].DeepEquals(news["foo"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["foo"].DeepEquals(news["foo"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"foo"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	inputsA := &testResourceInputs{Foo: pulumi.String("foo")}

	dbrValue, dbrA := true, (*bool)(nil)
	getDbr := func() bool {
		if dbrA == nil {
			return false
		}
		return *dbrA
	}

	var stackURN, provURN, urnA resource.URN = "urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
		"urn:pulumi:test::test::pulumi:providers:pkgA::provA", "urn:pulumi:test::test::pkgA:m:typA::resA"
	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			provider := &pulumi.ProviderResourceState{}
			err := ctx.RegisterResource(string(providers.MakeProviderType("pkgA")), "provA", nil, provider)
			assert.NoError(t, err)

			var res pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgA:m:typA", "resA", inputsA, &res,
				pulumi.Provider(provider), pulumi.DeleteBeforeReplace(getDbr()))
			assert.NoError(t, err)

			return nil
		})

	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	// Change the value of resA.A. Should create before replace
	inputsA.Foo = pulumi.String("bar")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: stackURN},
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it requires delete-before-replace and change the value of resA.A.
	// replacement should be delete-before-replace.
	dbrA, inputsA.Foo = &dbrValue, pulumi.String("baz")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: stackURN},
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	p.Run(t, snap)
}

func TestReadResourceGolangLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					assert.Equal(t, resource.ID("someId"), id)
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	var stackURN, defaultProviderURN, urnA resource.URN = "urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
		"urn:pulumi:test::test::pulumi:providers:pkgA::default", "urn:pulumi:test::test::pkgA:m:typA::resA"

	setupAndRunProgram := func() *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
				Project:     info.Project,
				Stack:       info.Stack,
				Parallel:    info.Parallel,
				DryRun:      info.DryRun,
				MonitorAddr: info.MonitorAddress,
			})
			assert.NoError(t, err)

			return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
				var res pulumi.CustomResourceState
				err := ctx.ReadResource("pkgA:m:typA", "resA", pulumi.ID("someId"), nil, &res)
				assert.NoError(t, err)

				return nil
			})
		})

		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						evts []Event, res result.Result) result.Result {

						assert.Nil(t, res)

						AssertSameSteps(t, []StepSummary{
							{Op: deploy.OpCreate, URN: stackURN},
							{Op: deploy.OpCreate, URN: defaultProviderURN},
							{Op: deploy.OpRead, URN: urnA},
						}, SuccessfulSteps(entries))

						return res
					},
				},
			},
		}
		return p.Run(t, nil)
	}

	setupAndRunProgram()
}

// ensures that RegisterResource, ReadResource (TODO https://github.com/pulumi/pulumi/issues/3562),
// and Invoke all respect the provider hierarchy
// most specific providers are used first 1. resource.provider, 2. resource.providers, 3. resource.parent.providers
func TestProviderInheritanceGolangLifecycle(t *testing.T) {
	type invokeArgs struct {
		Bang string `pulumi:"bang"`
		Bar  string `pulumi:"bar"`
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			v.InvokeF = func(tok tokens.ModuleMember,
				inputs resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
				assert.True(t, v.Config.DeepEquals(inputs))
				return nil, nil, nil
			}
			return v, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			v.InvokeF = func(tok tokens.ModuleMember,
				inputs resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
				assert.True(t, v.Config.DeepEquals(inputs))
				return nil, nil, nil
			}
			return v, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			// register a couple of providers, pass in some props that we can use to indentify it during invoke
			var providerA pulumi.ProviderResourceState
			err := ctx.RegisterResource(string(providers.MakeProviderType("pkgA")), "prov1",
				&testResourceInputs{
					Foo: pulumi.String("1"),
				}, &providerA)
			assert.NoError(t, err)
			var providerB pulumi.ProviderResourceState
			err = ctx.RegisterResource(string(providers.MakeProviderType("pkgB")), "prov2",
				&testResourceInputs{
					Bar:  pulumi.String("2"),
					Bang: pulumi.String(""),
				}, &providerB)
			assert.NoError(t, err)
			var providerBOverride pulumi.ProviderResourceState
			err = ctx.RegisterResource(string(providers.MakeProviderType("pkgB")), "prov3",
				&testResourceInputs{
					Bar:  pulumi.String(""),
					Bang: pulumi.String("3"),
				}, &providerBOverride)
			assert.NoError(t, err)
			parentProviders := make(map[string]pulumi.ProviderResource)
			parentProviders["pkgA"] = &providerA
			parentProviders["pkgB"] = &providerB
			// create a parent resource that uses provider map
			var parentResource pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgA:m:typA", "resA", nil, &parentResource, pulumi.ProviderMap(parentProviders))
			assert.NoError(t, err)
			// parent uses specified provider from map
			parentResultProvider := parentResource.GetProvider("pkgA:m:typA")
			assert.Equal(t, &providerA, parentResultProvider)

			// create a child resource
			var childResource pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgB:m:typB", "resBChild", nil, &childResource, pulumi.Parent(&parentResource))
			assert.NoError(t, err)

			// child uses provider value from parent
			childResultProvider := childResource.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerB, childResultProvider)

			// create a child with a provider specified
			var childWithOverride pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgB:m:typB", "resBChildOverride", nil, &childWithOverride,
				pulumi.Parent(&parentResource), pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			// child uses the specified provider, and not the provider from the parent
			childWithOverrideProvider := childWithOverride.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerBOverride, childWithOverrideProvider)

			// pass in a fake ID
			testID := pulumi.ID("testID")

			// read a resource that uses provider map
			err = ctx.ReadResource("pkgA:m:typA", "readResA", testID, nil, &parentResource, pulumi.ProviderMap(parentProviders))
			assert.NoError(t, err)
			// parent uses specified provider from map
			parentResultProvider = parentResource.GetProvider("pkgA:m:typA")
			assert.Equal(t, &providerA, parentResultProvider)

			// read a child resource
			err = ctx.ReadResource("pkgB:m:typB", "readResBChild", testID, nil, &childResource, pulumi.Parent(&parentResource))
			assert.NoError(t, err)

			// child uses provider value from parent
			childResultProvider = childResource.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerB, childResultProvider)

			// read a child with a provider specified
			err = ctx.ReadResource("pkgB:m:typB", "readResBChildOverride", testID, nil, &childWithOverride,
				pulumi.Parent(&parentResource), pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			// child uses the specified provider, and not the provider from the parent
			childWithOverrideProvider = childWithOverride.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerBOverride, childWithOverrideProvider)

			// invoke with specific provider
			var invokeResult struct{}
			err = ctx.Invoke("pkgB:do:something", invokeArgs{
				Bang: "3",
			}, &invokeResult, pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			// invoke with parent
			err = ctx.Invoke("pkgB:do:something", invokeArgs{
				Bar: "2",
			}, &invokeResult, pulumi.Parent(&parentResource))
			assert.NoError(t, err)

			// invoke with parent and provider
			err = ctx.Invoke("pkgB:do:something", invokeArgs{
				Bang: "3",
			}, &invokeResult, pulumi.Parent(&parentResource), pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			return nil
		})
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update}},
	}
	p.Run(t, nil)
}

func TestSingleComponentDefaultProviderLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor,
				typ, name string, parent resource.URN, inputs resource.PropertyMap,
				options plugin.ConstructOptions) (plugin.ConstructResult, error) {

				urn, _, _, err := monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{
					Parent:  parent,
					Aliases: options.Aliases,
					Protect: options.Protect,
				})
				assert.NoError(t, err)

				_, _, _, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
					Parent: urn,
				})
				assert.NoError(t, err)

				outs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
				err = monitor.RegisterResourceOutputs(urn, outs)
				assert.NoError(t, err)

				return plugin.ConstructResult{
					URN:     urn,
					Outputs: outs,
				}, nil
			}

			return &deploytest.Provider{
				ConstructF: construct,
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}, state)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 3),
	}
	p.Run(t, nil)
}

func TestResourceReferences(t *testing.T) {
	var urnA resource.URN
	var idA resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap,
					timeout float64, preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					if urn.Name() == "resB" {
						propA, ok := news["resA"]
						assert.True(t, ok)
						assert.Equal(t, resource.MakeResourceReference(urnA, idA, true, ""), propA)
					}

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			return v, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		urnA, idA, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		ins := resource.PropertyMap{
			"resA": resource.MakeResourceReference(urnA, idA, true, ""),
		}
		_, _, props, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)
		assert.Equal(t, ins, props)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 3),
	}
	p.Run(t, nil)
}

type updateContext struct {
	*deploytest.ResourceMonitor

	resmon       chan *deploytest.ResourceMonitor
	programErr   chan error
	snap         chan *deploy.Snapshot
	updateResult chan result.Result
}

func startUpdate(host plugin.Host) (*updateContext, error) {
	ctx := &updateContext{
		resmon:       make(chan *deploytest.ResourceMonitor),
		programErr:   make(chan error),
		snap:         make(chan *deploy.Snapshot),
		updateResult: make(chan result.Result),
	}

	stop := make(chan bool)
	port, _, err := rpcutil.Serve(0, stop, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, ctx)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Runtime: "client",
		RuntimeOptions: map[string]interface{}{
			"address": fmt.Sprintf("127.0.0.1:%d", port),
		},
	}

	go func() {
		snap, res := TestOp(Update).Run(p.GetProject(), p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
		ctx.snap <- snap
		close(ctx.snap)
		ctx.updateResult <- res
		close(ctx.updateResult)
		stop <- true
	}()

	ctx.ResourceMonitor = <-ctx.resmon
	return ctx, nil
}

func (ctx *updateContext) Finish(err error) (*deploy.Snapshot, result.Result) {
	ctx.programErr <- err
	close(ctx.programErr)

	return <-ctx.snap, <-ctx.updateResult
}

func (ctx *updateContext) GetRequiredPlugins(_ context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (ctx *updateContext) Run(_ context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// Connect to the resource monitor and create an appropriate client.
	conn, err := grpc.Dial(
		req.MonitorAddress,
		grpc.WithInsecure(),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "could not connect to resource monitor")
	}
	defer contract.IgnoreClose(conn)

	// Fire up a resource monitor client
	ctx.resmon <- deploytest.NewResourceMonitor(pulumirpc.NewResourceMonitorClient(conn))
	close(ctx.resmon)

	// Wait for the program to terminate.
	if err := <-ctx.programErr; err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}, nil
	}
	return &pulumirpc.RunResponse{}, nil
}

func (ctx *updateContext) GetPluginInfo(_ context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func TestLanguageClient(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	update, err := startUpdate(deploytest.NewPluginHost(nil, nil, nil, loaders...))
	if err != nil {
		t.Fatalf("failed to start update: %v", err)
	}

	// Register resources, etc.
	_, _, _, err = update.RegisterResource("pkgA:m:typA", "resA", true)
	assert.NoError(t, err)

	snap, res := update.Finish(nil)
	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 2)
}

func TestSingleComponentGetResourceDefaultProviderLifecycle(t *testing.T) {
	var urnB resource.URN
	var idB resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor, typ, name string, parent resource.URN,
				inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {

				urn, _, _, err := monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{
					Parent:       parent,
					Protect:      options.Protect,
					Aliases:      options.Aliases,
					Dependencies: options.Dependencies,
				})
				assert.NoError(t, err)

				urnB, idB, _, err = monitor.RegisterResource("pkgA:m:typB", "resB", true, deploytest.ResourceOptions{
					Parent: urn,
					Inputs: resource.PropertyMap{
						"bar": resource.NewStringProperty("baz"),
					},
				})
				assert.NoError(t, err)

				return plugin.ConstructResult{
					URN: urn,
					Outputs: resource.PropertyMap{
						"foo": resource.NewStringProperty("bar"),
						"res": resource.MakeResourceReference(urnB, idB, true, ""),
					},
				}, nil
			}

			return &deploytest.Provider{
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", inputs, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
				ConstructF: construct,
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
			"res": resource.MakeResourceReference(urnB, idB, true, ""),
		}, state)

		result, _, err := monitor.Invoke("pulumi:pulumi:getResource", resource.PropertyMap{
			"urn": resource.NewStringProperty(string(urnB)),
		}, "", "")
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"urn": resource.NewStringProperty(string(urnB)),
			"id":  resource.NewStringProperty(string(idB)),
			"state": resource.NewObjectProperty(resource.PropertyMap{
				"bar": resource.NewStringProperty("baz"),
			}),
		}, result)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

const importSchema = `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
	"pkgA:m:typA": {
      "inputProperties": {
	    "foo": {
		  "type": "string"
		}
      },
      "properties": {
	    "foo": {
		  "type": "string"
		}
      }
    }
  }
}`

func TestImport(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(version int) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if olds["foo"].DeepEquals(news["foo"]) {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}

					return plugin.DiffResult{
						Changes: plugin.DiffSome,
						DetailedDiff: map[string]plugin.PropertyDiff{
							"foo": {Kind: plugin.DiffUpdate},
						},
					}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo": resource.NewStringProperty("bar"),
						},
						Outputs: resource.PropertyMap{
							"foo": resource.NewStringProperty("bar"),
						},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// Run an import.
	snap, res = ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}}).Run(project, p.GetTarget(snap), p.Options, false, p.BackendClient, nil)

	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 4)
}

func TestConfigSecrets(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	crypter := config.NewSymmetricCrypter(make([]byte, 32))
	secret, err := crypter.EncryptValue("hunter2")
	assert.NoError(t, err)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 2),
		Config: config.Map{
			config.MustMakeKey("pkgA", "secret"): config.NewSecureValue(secret),
		},
		Decrypter: crypter,
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	if !assert.Len(t, snap.Resources, 2) {
		return
	}

	provider := snap.Resources[0]
	assert.True(t, provider.Inputs["secret"].IsSecret())
	assert.True(t, provider.Outputs["secret"].IsSecret())
}
