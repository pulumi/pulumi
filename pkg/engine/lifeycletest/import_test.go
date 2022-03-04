package lifecycletest

import (
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestImportOption(t *testing.T) {
	t.Parallel()

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
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)

	// Run a second update after fixing the inputs. The import should succeed.
	inputs["foo"] = resource.NewStringProperty("bar")
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
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
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
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
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
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
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)

	// Finally, destroy the stack. The `Delete` function should be called.
	_, res = TestOp(Destroy).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
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
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
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
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
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
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
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
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
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
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
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
	t.Parallel()

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
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
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
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
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

func TestImportUpdatedID(t *testing.T) {
	t.Parallel()

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

const importSchema = `{
  "version": "0.0.1",
  "name": "pkgA",
  "resources": {
	"pkgA:m:typA": {
      "inputProperties": {
	    "foo": {
		  "type": "string"
		},
	    "frob": {
		  "type": "number"
		}
      },
	  "requiredInputs": [
		  "frob"
	  ],
      "properties": {
	    "foo": {
		  "type": "string"
		},
	    "frob": {
		  "type": "number"
		}
      }
    }
  }
}`

func diffImportResource(urn resource.URN, id resource.ID,
	olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

	if olds["foo"].DeepEquals(news["foo"]) && olds["frob"].DeepEquals(news["frob"]) {
		return plugin.DiffResult{Changes: plugin.DiffNone}, nil
	}

	detailedDiff := make(map[string]plugin.PropertyDiff)
	if !olds["foo"].DeepEquals(news["foo"]) {
		detailedDiff["foo"] = plugin.PropertyDiff{Kind: plugin.DiffUpdate}
	}
	if !olds["frob"].DeepEquals(news["frob"]) {
		detailedDiff["frob"] = plugin.PropertyDiff{Kind: plugin.DiffUpdate}
	}

	return plugin.DiffResult{
		Changes:      plugin.DiffSome,
		DetailedDiff: detailedDiff,
	}, nil
}

func TestImportPlan(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(version int) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
						Outputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
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
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// Run an import.
	snap, res = ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}}).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)

	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 4)
}

func TestImportIgnoreChanges(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
						Outputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo":  resource.NewStringProperty("foo"),
				"frob": resource.NewNumberProperty(1),
			},
			ImportID:      "import-id",
			IgnoreChanges: []string{"foo"},
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.NewStringProperty("bar"), snap.Resources[1].Outputs["foo"])
}

func TestImportPlanExistingImport(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(version int) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
						Outputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		stackURN, _, _, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo":  resource.NewStringProperty("bar"),
				"frob": resource.NewNumberProperty(1),
			},
			ImportID: "imported-id",
			Parent:   stackURN,
		})
		require.NoError(t, err)

		err = monitor.RegisterResourceOutputs(stackURN, resource.PropertyMap{})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// Run an import with a different ID. This should fail.
	_, res = ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resA",
		ID:   "imported-id-2",
	}}).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)

	// Run an import with a matching ID. This should succeed and do nothing.
	snap, res = ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resA",
		ID:   "imported-id",
	}}).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, _ result.Result) result.Result {
			for _, e := range entries {
				assert.Equal(t, e.Step.Op(), deploy.OpSame)
			}
			return nil
		})

	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 3)
}

func TestImportPlanEmptyState(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(version int) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
						Outputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}
	program := deploytest.NewLanguageRuntime(nil)
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	// Run the initial import.
	project := p.GetProject()
	snap, res := ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}}).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)

	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 3)
}

func TestImportPlanSpecificProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(version int) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
						Outputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	snap, res = ImportOp([]deploy.Import{{
		Type:     "pkgA:m:typA",
		Name:     "resB",
		ID:       "imported-id",
		Provider: p.NewProviderURN("pkgA", "provA", ""),
	}}).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)

	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 3)
}

func TestImportPlanSpecificProperties(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(version int) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
						Outputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
						},
					}, resource.StatusOK, nil
				},
				CheckF: func(
					urn resource.URN, olds, news resource.PropertyMap,
					sequenceNumber int) (resource.PropertyMap, []plugin.CheckFailure, error) {
					// Error unless "foo" and "frob" are in news

					if _, has := news["foo"]; !has {
						return nil, nil, errors.New("Need foo")
					}

					if _, has := news["frob"]; !has {
						return nil, nil, errors.New("Need frob")
					}

					return news, nil, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	snap, res = ImportOp([]deploy.Import{{
		Type:     "pkgA:m:typA",
		Name:     "resB",
		ID:       "imported-id",
		Provider: p.NewProviderURN("pkgA", "provA", ""),
	}}).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)

	// This should fail to import because the default behaviour only imports with required fields
	// (frob) but our check function needs foo and frob.
	assert.NotNil(t, res)
	assert.Len(t, snap.Resources, 2)

	// Try and import again specifying foo and frob
	snap, res = ImportOp([]deploy.Import{{
		Type:       "pkgA:m:typA",
		Name:       "resB",
		ID:         "imported-id",
		Provider:   p.NewProviderURN("pkgA", "provA", ""),
		Properties: []string{"foo", "frob"},
	}}).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)

	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 3)
}
