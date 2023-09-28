package lifecycletest

import (
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestSingleResourceDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)
}

func TestSingleResourceExplicitProviderLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)
}

func TestSingleResourceDefaultProviderUpgrade(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
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
		_ []Event, res result.Result,
	) result.Result {
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
		snap, err := entries.Snap(target.Snapshot)
		require.NoError(t, err)
		assert.Len(t, snap.Resources, 2)
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
			_ []Event, res result.Result,
		) result.Result {
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
			snap, err := entries.Snap(target.Snapshot)
			require.NoError(t, err)
			assert.Len(t, snap.Resources, 0)
			return res
		},
	}}
	p.Run(t, old)

	// Run a partial lifecycle using the base snapshot, skipping the initial update step.
	p.Steps = MakeBasicLifecycleSteps(t, 2)[1:]
	p.Run(t, old)
}

func TestSingleResourceDefaultProviderReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range newInputs {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
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
			_ []Event, res result.Result,
		) result.Result {
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
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range newInputs {
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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
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
			_ []Event, res result.Result,
		) result.Result {
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

type configurableProvider struct {
	id      string
	replace bool
	creates *sync.Map
	deletes *sync.Map
}

func (p *configurableProvider) configure(news resource.PropertyMap) error {
	p.id = news["id"].StringValue()
	return nil
}

func (p *configurableProvider) create(urn resource.URN, inputs resource.PropertyMap, timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	uid, err := uuid.NewV4()
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}
	id := resource.ID(uid.String())

	p.creates.Store(id, p.id)
	return id, inputs, resource.StatusOK, nil
}

func (p *configurableProvider) delete(urn resource.URN, id resource.ID, olds resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	p.deletes.Store(id, p.id)
	return resource.StatusOK, nil
}

// TestSingleResourceExplicitProviderAliasUpdateDelete verifies that providers respect aliases during updates, and
// that the correct instance of an explicit provider is used to delete a removed resource.
func TestSingleResourceExplicitProviderAliasUpdateDelete(t *testing.T) {
	t.Parallel()

	var creates, deletes sync.Map

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			configurable := &configurableProvider{
				creates: &creates,
				deletes: &deletes,
			}

			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil
				},
				ConfigureF: configurable.configure,
				CreateF:    configurable.create,
				DeleteF:    configurable.delete,
			}, nil
		}),
	}

	providerInputs := resource.PropertyMap{
		resource.PropertyKey("id"): resource.NewStringProperty("first"),
	}
	providerName := "provA"
	aliases := []resource.URN{}
	registerResource := true
	var resourceID resource.ID
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), providerName, true,
			deploytest.ResourceOptions{
				Inputs:    providerInputs,
				AliasURNs: aliases,
			})
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		if registerResource {
			_, resourceID, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Provider: provRef.String(),
			})
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its initial update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// Add a provider alias to the original URN.
	aliases = []resource.URN{
		p.NewProviderURN("pkgA", "provA", ""),
	}
	// Change the provider name and configuration and remove the resource. This will cause an Update for the provider
	// and a Delete for the resource. The updated provider instance should be used to perform the delete.
	providerName = "provB"
	providerInputs[resource.PropertyKey("id")] = resource.NewStringProperty("second")
	registerResource = false

	p.Steps = []TestStep{{Op: Update}}
	_ = p.Run(t, snap)

	// Check the identity of the provider that performed the delete.
	deleterID, ok := deletes.Load(resourceID)
	require.True(t, ok)
	assert.Equal(t, "second", deleterID)
}

// TestSingleResourceExplicitProviderAliasReplace verifies that providers respect aliases,
// and propagate replaces as a result of an aliased provider diff.
func TestSingleResourceExplicitProviderAliasReplace(t *testing.T) {
	t.Parallel()

	var creates, deletes sync.Map

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			configurable := &configurableProvider{
				replace: true,
				creates: &creates,
				deletes: &deletes,
			}

			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					keys := []resource.PropertyKey{}
					for k := range newInputs {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys}, nil
				},
				ConfigureF: configurable.configure,
				CreateF:    configurable.create,
				DeleteF:    configurable.delete,
			}, nil
		}),
	}

	providerInputs := resource.PropertyMap{
		resource.PropertyKey("id"): resource.NewStringProperty("first"),
	}
	providerName := "provA"
	aliases := []resource.URN{}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), providerName, true,
			deploytest.ResourceOptions{
				Inputs:    providerInputs,
				AliasURNs: aliases,
			})
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its no-op update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// add a provider alias to the original URN
	aliases = []resource.URN{
		p.NewProviderURN("pkgA", "provA", ""),
	}
	// change the provider name
	providerName = "provB"
	// run an update expecting no-op respecting the aliases.
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result,
		) result.Result {
			for _, entry := range entries {
				if entry.Step.Op() != deploy.OpSame {
					t.Fatalf("update should contain no changes: %v", entry.Step.URN())
				}
			}
			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the config and run an update maintaining the alias. We expect everything to require replacement.
	providerInputs[resource.PropertyKey("id")] = resource.NewStringProperty("second")
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result,
		) result.Result {
			provURN := p.NewProviderURN("pkgA", providerName, "")
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Find the delete and create IDs for the resource.
			var createdID, deletedID resource.ID

			// Look for replace steps on the provider and the resource.
			replacedProvider, replacedResource := false, false
			for _, entry := range entries {
				op := entry.Step.Op()

				if entry.Step.URN() == resURN {
					switch op {
					case deploy.OpCreateReplacement:
						createdID = entry.Step.New().ID
					case deploy.OpDeleteReplaced:
						deletedID = entry.Step.Old().ID
					}
				}

				if entry.Kind != JournalEntrySuccess || op != deploy.OpDeleteReplaced {
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

			// Check the identities of the providers that performed the create and delete.
			//
			// For a replacement, the newly-created provider should be used to create the new resource, and the original
			// provider should be used to delete the old resource.
			creatorID, ok := creates.Load(createdID)
			require.True(t, ok)
			assert.Equal(t, "second", creatorID)

			deleterID, ok := deletes.Load(deletedID)
			require.True(t, ok)
			assert.Equal(t, "first", deleterID)

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Resume the lifecycle with another no-op update.
	p.Steps = steps[2:]
	p.Run(t, snap)
}

func TestSingleResourceExplicitProviderDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range newInputs {
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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
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
			_ []Event, res result.Result,
		) result.Result {
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

// TestDefaultProviderDiff tests that the engine can gracefully recover whenever a resource's default provider changes
// and there is no diff in the provider's inputs.
func TestDefaultProviderDiff(t *testing.T) {
	t.Parallel()

	const resName, resBName = "resA", "resB"
	expect1710 := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.10"), func() (plugin.Provider, error) {
			// If we don't expect to load this assert if called
			if !expect1710 {
				assert.Fail(t, "unexpected call to 0.17.10 provider")
			}
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.11"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.12"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	runProgram := func(base *deploy.Snapshot, versionA, versionB string, expectedStep display.StepOp) *deploy.Snapshot {
		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &TestPlan{
			Options: TestUpdateOptions{HostF: hostF},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result,
					) result.Result {
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
	expect1710 = false
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
	t.Parallel()

	const resName, resBName = "resA", "resB"
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.17.10"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// This implementation of DiffConfig always requests replacement.
				DiffConfigF: func(_ resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					keys := []resource.PropertyKey{}
					for k := range newInputs {
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

	runProgram := func(base *deploy.Snapshot, versionA, versionB string,
		expectedSteps ...display.StepOp,
	) *deploy.Snapshot {
		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &TestPlan{
			Options: TestUpdateOptions{HostF: hostF},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result,
					) result.Result {
						for _, entry := range entries {
							if entry.Kind != JournalEntrySuccess {
								continue
							}

							switch entry.Step.URN().Name().String() {
							case resName:
								assert.Subset(t, expectedSteps, []display.StepOp{entry.Step.Op()})
							case resBName:
								assert.Subset(t,
									[]display.StepOp{deploy.OpCreate, deploy.OpSame}, []display.StepOp{entry.Step.Op()})
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

func TestProviderVersionDefault(t *testing.T) {
	t.Parallel()

	version := ""
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			version = "1.0.0"
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.5.0"), func() (plugin.Provider, error) {
			version = "1.5.0"
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)

	assert.Equal(t, "1.5.0", version)
}

func TestProviderVersionOption(t *testing.T) {
	t.Parallel()

	version := ""
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			version = "1.0.0"
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.5.0"), func() (plugin.Provider, error) {
			version = "1.5.0"
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true,
			deploytest.ResourceOptions{
				Version: "1.0.0",
			})
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)

	assert.Equal(t, "1.0.0", version)
}

func TestProviderVersionInput(t *testing.T) {
	t.Parallel()

	version := ""
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			version = "1.0.0"
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.5.0"), func() (plugin.Provider, error) {
			version = "1.5.0"
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true,
			deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"version": resource.NewStringProperty("1.0.0"),
				},
			})
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)

	assert.Equal(t, "1.0.0", version)
}

func TestProviderVersionInputAndOption(t *testing.T) {
	t.Parallel()

	version := ""
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			version = "1.0.0"
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.5.0"), func() (plugin.Provider, error) {
			version = "1.5.0"
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true,
			deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"version": resource.NewStringProperty("1.5.0"),
				},
				Version: "1.0.0",
			})
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)

	assert.Equal(t, "1.0.0", version)
}

func TestPluginDownloadURLPassthrough(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	pkgAPluginDownloadURL := "get.pulumi.com/${VERSION}"
	pkgAType := providers.MakeProviderType("pkgA")

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(pkgAType, "provA", true, deploytest.ResourceOptions{
			PluginDownloadURL: pkgAPluginDownloadURL,
		})
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	steps := MakeBasicLifecycleSteps(t, 2)
	steps[0].ValidateAnd(func(project workspace.Project, target deploy.Target, entries JournalEntries,
		_ []Event, res result.Result,
	) result.Result {
		for _, e := range entries {
			r := e.Step.New()
			if r.Type == pkgAType && r.Inputs["pluginDownloadURL"].StringValue() != pkgAPluginDownloadURL {
				return result.Errorf("Found unexpected value %v", r.Inputs["pluginDownloadURL"])
			}
		}
		return nil
	})
	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   steps,
	}
	p.Run(t, nil)
}

// Check that creating a resource with pluginDownloadURL set will instantiate a default provider with
// pluginDownloadURL set.
func TestPluginDownloadURLDefaultProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	url := "get.pulumi.com"

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA::Foo", "foo", true, deploytest.ResourceOptions{
			PluginDownloadURL: url,
		})
		return err
	})

	snapshot := (&TestPlan{
		Options: TestUpdateOptions{HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)},
		// The first step is the update. We don't want the full lifecycle because we want to see the
		// created resources.
		Steps: MakeBasicLifecycleSteps(t, 2)[:1],
	}).Run(t, nil)

	foundDefaultProvider := false
	for _, r := range snapshot.Resources {
		if providers.IsDefaultProvider(r.URN) {
			actualURL, err := providers.GetProviderDownloadURL(r.Inputs)
			assert.NoError(t, err)
			assert.Equal(t, url, actualURL)
			foundDefaultProvider = true
		}
	}
	assert.Truef(t, foundDefaultProvider, "Found resources: %#v", snapshot.Resources)
}

func TestMultipleResourceDenyDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		f          deploytest.ProgramFunc
		disabled   string
		expectFail bool
	}{
		{
			name: "default-blocked",
			f: func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
				assert.NoError(t, err)
				_, _, _, err = monitor.RegisterResource("pkgB:m:typB", "resB", true)
				assert.NoError(t, err)

				return nil
			},
			disabled:   `["pkgA"]`,
			expectFail: true,
		},
		{
			name: "explicit-not-blocked",
			f: func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
				assert.NoError(t, err)
				provRef, err := providers.NewReference(provURN, provID)
				assert.NoError(t, err)

				_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
					Provider: provRef.String(),
				})
				assert.NoError(t, err)

				_, _, _, err = monitor.RegisterResource("pkgB:m:typB", "resB", true)
				assert.NoError(t, err)

				return nil
			},
			disabled:   `["pkgA"]`,
			expectFail: false,
		},
		{
			name: "wildcard",
			f: func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
				assert.NoError(t, err)
				_, _, _, err = monitor.RegisterResource("pkgB:m:typB", "resB", true)
				assert.NoError(t, err)

				return nil
			},
			disabled:   `["*"]`,
			expectFail: true,
		},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{}, nil
				}),
				deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{}, nil
				}),
			}

			programF := deploytest.NewLanguageRuntimeF(tt.f)
			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

			c := config.Map{}
			k := config.MustMakeKey("pulumi", "disable-default-providers")
			c[k] = config.NewValue(tt.disabled)

			expectedCreated := 4
			if tt.expectFail {
				expectedCreated = 0
			}
			update := MakeBasicLifecycleSteps(t, expectedCreated)[:1]
			update[0].ExpectFailure = tt.expectFail
			p := &TestPlan{
				Options: TestUpdateOptions{HostF: hostF},
				Steps:   update,
				Config:  c,
			}
			p.Run(t, nil)
		})
	}
}

func TestProviderVersionAssignment(t *testing.T) {
	t.Parallel()

	prog := func(opts ...deploytest.ResourceOptions) deploytest.ProgramFunc {
		return func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:r:typA", "resA", true, opts...)
			if err != nil {
				return err
			}
			_, _, _, err = monitor.RegisterResource("pulumi:providers:pkgA", "provA", true, opts...)
			if err != nil {
				return err
			}
			return nil
		}
	}

	cases := []struct {
		name     string
		plugins  []workspace.PluginSpec
		snapshot *deploy.Snapshot
		validate func(t *testing.T, r *resource.State)
		versions []string
		prog     deploytest.ProgramFunc
	}{
		{
			name:     "empty",
			versions: []string{"1.0.0"},
			validate: func(*testing.T, *resource.State) {},
			prog:     prog(),
		},
		{
			name:     "default-version",
			versions: []string{"1.0.0", "1.1.0"},
			plugins: []workspace.PluginSpec{{
				Name:              "pkgA",
				Version:           &semver.Version{Major: 1, Minor: 1},
				PluginDownloadURL: "example.com/default",
				Kind:              workspace.ResourcePlugin,
			}},
			validate: func(t *testing.T, r *resource.State) {
				if providers.IsProviderType(r.Type) && !providers.IsDefaultProvider(r.URN) {
					assert.Equal(t, r.Inputs["version"].StringValue(), "1.1.0")
					assert.Equal(t, r.Inputs["pluginDownloadURL"].StringValue(), "example.com/default")
				}
			},
			prog: prog(),
		},
		{
			name:     "specified-provider",
			versions: []string{"1.0.0", "1.1.0"},
			plugins: []workspace.PluginSpec{{
				Name:    "pkgA",
				Version: &semver.Version{Major: 1, Minor: 1},
				Kind:    workspace.ResourcePlugin,
			}},
			validate: func(t *testing.T, r *resource.State) {
				if providers.IsProviderType(r.Type) && !providers.IsDefaultProvider(r.URN) {
					_, hasVersion := r.Inputs["version"]
					assert.False(t, hasVersion)
					assert.Equal(t, r.Inputs["pluginDownloadURL"].StringValue(), "example.com/download")
				}
			},
			prog: prog(deploytest.ResourceOptions{PluginDownloadURL: "example.com/download"}),
		},
		{
			name:     "higher-in-snapshot",
			versions: []string{"1.3.0", "1.1.0"},
			prog:     prog(),
			plugins: []workspace.PluginSpec{{
				Name:    "pkgA",
				Version: &semver.Version{Major: 1, Minor: 1},
				Kind:    workspace.ResourcePlugin,
			}},
			snapshot: &deploy.Snapshot{
				Resources: []*resource.State{
					{
						Type: "providers:pulumi:pkgA",
						URN:  "this:is:a:urn::ofaei",
						Inputs: map[resource.PropertyKey]resource.PropertyValue{
							"version": resource.NewPropertyValue("1.3.0"),
						},
					},
				},
			},
			validate: func(t *testing.T, r *resource.State) {
				if providers.IsProviderType(r.Type) && !providers.IsDefaultProvider(r.URN) {
					assert.Equal(t, r.Inputs["version"].StringValue(), "1.1.0")
				}
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			programF := deploytest.NewLanguageRuntimeF(c.prog, c.plugins...)
			loaders := []*deploytest.ProviderLoader{}
			for _, v := range c.versions {
				loaders = append(loaders,
					deploytest.NewProviderLoader("pkgA", semver.MustParse(v), func() (plugin.Provider, error) {
						return &deploytest.Provider{}, nil
					}))
			}
			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

			update := []TestStep{{Op: Update, Validate: func(
				project workspace.Project, target deploy.Target, entries JournalEntries,
				events []Event, res result.Result,
			) result.Result {
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				assert.Len(t, snap.Resources, 3)
				for _, r := range snap.Resources {
					c.validate(t, r)
				}
				return nil
			}}}

			p := &TestPlan{
				Options: TestUpdateOptions{HostF: hostF},
				Steps:   update,
			}
			p.Run(t, &deploy.Snapshot{})
		})
	}
}

// TestDeletedWithOptionInheritance checks that a resource that sets its parent to another resource inherits
// that resource's DeletedWith option.
func TestDeletedWithOptionInheritance(t *testing.T) {
	t.Parallel()

	expectedUrn := resource.CreateURN("expect-this", "pkg:index:type", "", "project", "stack")

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		parentUrn, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			DeletedWith: expectedUrn,
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Parent: parentUrn,
		})
		assert.NoError(t, err)

		return nil
	})

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(
					urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	for _, res := range snap.Resources[1:] {
		assert.Equal(t, expectedUrn, res.DeletedWith)
	}
	assert.Nil(t, res)
}

// TestDeletedWithOptionInheritanceMLC checks that an MLC's DeletedWith option is propagated to resources that
// set an MLC as its parent. MLC's are remote and at the time of writing their RegisterResource call asks the
// resource monitor to ask the constructor to call the necessary RegisterResource calls on the program's behalf.
func TestDeletedWithOptionInheritanceMLC(t *testing.T) {
	t.Parallel()

	expectedUrn := resource.CreateURN("expect-this", "pkg:index:type", "", "project", "stack")

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		parentUrn, _, _, err := monitor.RegisterResource("pkgA:m:typComponent", "resA", false, deploytest.ResourceOptions{
			Remote:      true,
			DeletedWith: expectedUrn,
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Parent: parentUrn,
		})
		assert.NoError(t, err)

		return nil
	})

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(
					urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil
				},
				ConstructF: func(monitor *deploytest.ResourceMonitor, typ, name string,
					parent resource.URN, inputs resource.PropertyMap,
					info plugin.ConstructInfo, options plugin.ConstructOptions,
				) (plugin.ConstructResult, error) {
					require.Equal(t, "resA", name)
					require.Equal(t, "pkgA:m:typComponent", typ)

					urn, _, _, err := monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{
						DeletedWith: options.DeletedWith,
					})
					require.NoError(t, err)

					_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
						Parent: urn,
					})
					require.NoError(t, err)
					return plugin.ConstructResult{
						URN: urn,
					}, nil
				},
			}, nil
		}),
	}

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	for _, res := range snap.Resources[1:] {
		assert.Equal(t, expectedUrn, res.DeletedWith)
	}
	assert.Nil(t, res)
}
