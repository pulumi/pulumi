package lifecycletest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestImportOption(t *testing.T) {
	t.Parallel()

	readInputs := resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	readOutputs := resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
		"out": resource.NewNumberProperty(41),
	}

	// For imports we expect inputs and state to be nil, but when we change to do a read they should both be set to the
	// resource inputs.
	var expectedInputs, expectedState resource.PropertyMap
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ resource.URN, _ resource.ID,
					_, oldOutputs, newInputs resource.PropertyMap, _ []string,
				) (plugin.DiffResult, error) {
					if oldOutputs["foo"].DeepEquals(newInputs["foo"]) {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}

					diffKind := plugin.DiffUpdate
					if newInputs["foo"].IsString() && newInputs["foo"].StringValue() == "replace" {
						diffKind = plugin.DiffUpdateReplace
					}

					return plugin.DiffResult{
						Changes: plugin.DiffSome,
						DetailedDiff: map[string]plugin.PropertyDiff{
							"foo": {Kind: diffKind},
						},
					}, nil
				},
				CreateF: func(_ resource.URN, news resource.PropertyMap, _ float64,
					_ bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(_ resource.URN, _ resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					assert.Equal(t, expectedInputs, inputs)
					assert.Equal(t, expectedState, state)

					return plugin.ReadResult{
						Inputs:  readInputs,
						Outputs: readOutputs,
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	readID, importID, inputs := resource.ID(""), resource.ID("id"), resource.PropertyMap{}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		if readID != "" {
			_, _, err = monitor.ReadResource("pkgA:m:typA", "resA", readID, "", inputs, "", "", "")
		} else {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:   inputs,
				ImportID: importID,
			})
		}
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}
	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update. The import should fail due to a mismatch in inputs between the program and the
	// actual resource state.
	project := p.GetProject()
	_, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.ErrorContains(t, err, "step application failed: inputs to import do not match the existing resource")

	// Run a second update after fixing the inputs. The import should succeed.
	inputs["foo"] = resource.NewStringProperty("bar")
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
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
			return err
		}, "1")
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, readInputs, snap.Resources[1].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[1].Outputs)

	// Now, run another update. The update should succeed and there should be no diffs.
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return err
		}, "2")
	assert.NoError(t, err)
	assert.Equal(t, readInputs, snap.Resources[1].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[1].Outputs)

	// Change a property value and run a third update. The update should succeed.
	inputs["foo"] = resource.NewStringProperty("rab")
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
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
			return err
		}, "3")
	assert.NoError(t, err)
	// This should call update not read, which just returns the passed inputs as outputs.
	assert.Equal(t, inputs, snap.Resources[1].Inputs)
	assert.Equal(t, inputs, snap.Resources[1].Outputs)

	// Change the property value s.t. the resource requires replacement. The update should fail.
	inputs["foo"] = resource.NewStringProperty("replace")
	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "4")
	assert.ErrorContains(t, err, "reviously-imported resources that still specify an ID may not be replaced")

	// Finally, destroy the stack. The `Delete` function should be called.
	_, err = TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return err
		}, "5")
	assert.NoError(t, err)

	// Now clear the ID to import and run an initial update to create a resource that we will import-replace.
	importID, inputs["foo"] = "", resource.NewStringProperty("bar")
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return err
		}, "6")
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 2)
	// This will have just called create which returns the inputs as outputs.
	assert.Equal(t, inputs, snap.Resources[1].Inputs)
	assert.Equal(t, inputs, snap.Resources[1].Outputs)

	// Set the import ID to the same ID as the existing resource and run an update. This should produce no changes.
	for _, r := range snap.Resources {
		if r.URN == resURN {
			importID = r.ID
		}
	}
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return err
		}, "7")
	assert.NoError(t, err)
	// This will have 'same'd so the inputs and outputs will be the same as the lat run with create.
	assert.Equal(t, inputs, snap.Resources[1].Inputs)
	assert.Equal(t, inputs, snap.Resources[1].Outputs)

	// Then set the import ID and run another update. The update should succeed and should show an import-replace and
	// a delete-replaced.
	importID = "id"
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
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
			return err
		}, "8")
	assert.NoError(t, err)
	assert.Equal(t, readInputs, snap.Resources[1].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[1].Outputs)

	// Change the program to read a resource rather than creating one.
	readID = "id"
	expectedInputs, expectedState = inputs, inputs
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
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
			return err
		}, "9")
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, readInputs, snap.Resources[1].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[1].Outputs)

	// Now have the program import the resource. We should see an import-replace and a read-discard.
	readID, importID = "", readID
	expectedInputs, expectedState = nil, nil
	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
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
			return err
		}, "10")
	assert.NoError(t, err)
	assert.Equal(t, readInputs, snap.Resources[1].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[1].Outputs)
}

// TestImportWithDifferingImportIdentifierFormat tests importing a resource that has a different format of identifier
// for the import input than for the ID property, ensuring that a second update does not result in a replace.
func TestImportWithDifferingImportIdentifierFormat(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if oldOutputs["foo"].DeepEquals(newInputs["foo"]) {
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
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
			// The import ID is deliberately not the same as the ID returned from Read.
			ImportID: resource.ID("import-id"),
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}
	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update. The import should succeed.
	project := p.GetProject()
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
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
			return err
		}, "0")
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 2)

	// Now, run another update. The update should succeed and there should be no diffs.
	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, err error) error {
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			return err
		}, "1")
	assert.NoError(t, err)
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			ImportID: importID,
		})
		assert.NoError(t, err)
		assert.Equal(t, actualID, resp.ID)
		return nil
	})
	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.T = t

	p.Steps = []TestStep{{Op: Refresh, SkipPreview: true}}

	// Refresh requires at least one resource in order to proceed.
	stackURN := resource.URN("urn:pulumi:stack::stack::pulumi:pulumi:Stack::foo")
	stackResource := newResource(
		stackURN,
		"",
		"foo",
		"",
		nil,
		nil,
		nil,
		false,
	)
	snap := p.Run(t, &deploy.Snapshot{Resources: []*resource.State{stackResource}})

	require.NotEmpty(t, snap.Resources)

	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN, stackURN:
			// continue
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
	oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
) (plugin.DiffResult, error) {
	if oldOutputs["foo"].DeepEquals(newInputs["foo"]) && oldOutputs["frob"].DeepEquals(newInputs["frob"]) {
		return plugin.DiffResult{Changes: plugin.DiffNone}, nil
	}

	detailedDiff := make(map[string]plugin.PropertyDiff)
	if !oldOutputs["foo"].DeepEquals(newInputs["foo"]) {
		detailedDiff["foo"] = plugin.PropertyDiff{Kind: plugin.DiffUpdate}
	}
	if !oldOutputs["frob"].DeepEquals(newInputs["frob"]) {
		detailedDiff["frob"] = plugin.PropertyDiff{Kind: plugin.DiffUpdate}
	}

	return plugin.DiffResult{
		Changes:      plugin.DiffSome,
		DetailedDiff: detailedDiff,
	}, nil
}

func TestImportPlan(t *testing.T) {
	t.Parallel()

	readInputs := resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	readOutputs := resource.PropertyMap{
		"foo":  resource.NewStringProperty("bar"),
		"frob": resource.NewNumberProperty(1),
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{
						ID:      "actual-id",
						Inputs:  readInputs,
						Outputs: readOutputs,
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Run an import.
	snap, err = ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}}).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 4)

	// Import should save the ID, inputs and outputs
	assert.Equal(t, resource.ID("actual-id"), snap.Resources[3].ID)
	assert.Equal(t, readInputs, snap.Resources[3].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[3].Outputs)

	// Import should set Created and Modified timestamps on state.
	for _, r := range snap.Resources {
		assert.NotNil(t, r.Created)
		assert.NotNil(t, r.Modified)
	}
}

func TestImportIgnoreChanges(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
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
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.NewStringProperty("bar"), snap.Resources[1].Outputs["foo"])
}

func TestImportPlanExistingImport(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo":  resource.NewStringProperty("bar"),
				"frob": resource.NewNumberProperty(1),
			},
			ImportID: "imported-id",
			Parent:   resp.URN,
		})
		require.NoError(t, err)

		err = monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Run an import with a different ID. This should fail.
	_, err = ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resA",
		ID:   "imported-id-2",
	}}).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.Error(t, err)

	// Run an import with a matching ID. This should succeed and do nothing.
	snap, err = ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resA",
		ID:   "imported-id",
	}}).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, entries JournalEntries, _ []Event, _ error) error {
			for _, e := range entries {
				assert.Equal(t, deploy.OpSame, e.Step.Op())
			}
			return nil
		}, "2")

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
}

func TestImportPlanEmptyState(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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
	programF := deploytest.NewLanguageRuntimeF(nil)
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial import.
	project := p.GetProject()
	snap, err := ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}}).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
}

func TestImportPlanSpecificProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	snap, err = ImportOp([]deploy.Import{{
		Type:     "pkgA:m:typA",
		Name:     "resB",
		ID:       "imported-id",
		Provider: p.NewProviderURN("pkgA", "provA", ""),
	}}).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
}

func TestImportPlanSpecificProperties(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{
						Inputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
							"baz":  resource.NewNumberProperty(2),
						},
						Outputs: resource.PropertyMap{
							"foo":  resource.NewStringProperty("bar"),
							"frob": resource.NewNumberProperty(1),
							"baz":  resource.NewNumberProperty(2),
						},
					}, resource.StatusOK, nil
				},
				CheckF: func(
					urn resource.URN, olds, news resource.PropertyMap,
					randomSeed []byte,
				) (resource.PropertyMap, []plugin.CheckFailure, error) {
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Import specifying to use just foo and frob
	snap, err = ImportOp([]deploy.Import{{
		Type:       "pkgA:m:typA",
		Name:       "resB",
		ID:         "imported-id",
		Provider:   p.NewProviderURN("pkgA", "provA", ""),
		Properties: []string{"foo", "frob"},
	}}).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 3)

	// We should still have the baz output but will be missing its input
	assert.Equal(t, resource.NewNumberProperty(2), snap.Resources[2].Outputs["baz"])
	assert.NotContains(t, snap.Resources[2].Inputs, "baz")
}

// Test that we can import one resource, then import another resource with the first one as a parent.
func TestImportIntoParent(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "", news, resource.StatusUnknown, errors.New("not implemented")
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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
	programF := deploytest.NewLanguageRuntimeF(nil)
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial import.
	project := p.GetProject()
	snap, err := ImportOp([]deploy.Import{
		{
			Type:   "pkgA:m:typA",
			Name:   "resB",
			ID:     "imported-idB",
			Parent: p.NewURN("pkgA:m:typA", "resA", ""),
		},
		{
			Type: "pkgA:m:typA",
			Name: "resA",
			ID:   "imported-idA",
		},
	}).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 4)
}

func TestImportComponent(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "", nil, resource.StatusUnknown, errors.New("not implemented")
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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
	programF := deploytest.NewLanguageRuntimeF(nil)
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial import.
	project := p.GetProject()
	snap, err := ImportOp([]deploy.Import{
		{
			Type:      "my-component",
			Name:      "comp",
			Component: true,
		},
		{
			Type:   "pkgA:m:typA",
			Name:   "resB",
			ID:     "imported-id",
			Parent: p.NewURN("my-component", "comp", ""),
		},
	}).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 4)

	// Ensure that the resource 2 is the component.
	comp := snap.Resources[2]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::my-component::comp"), comp.URN)
	// Ensure it's marked as a component.
	assert.False(t, comp.Custom, "expected component resource to not be marked as custom")

	// Ensure resource 3 is the custom resource
	custom := snap.Resources[3]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::my-component$pkgA:m:typA::resB"), custom.URN)
	// Ensure it's marked as custom.
	assert.True(t, custom.Custom, "expected custom resource to be marked as custom")
}

func TestImportRemoteComponent(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("mlc", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: diffImportResource,
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "", nil, resource.StatusUnknown, errors.New("not implemented")
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
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
	programF := deploytest.NewLanguageRuntimeF(nil)
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial import.
	project := p.GetProject()
	snap, err := ImportOp([]deploy.Import{
		{
			Type:      "mlc:index:Component",
			Name:      "comp",
			Component: true,
			Remote:    true,
		},
		{
			Type:   "pkgA:m:typA",
			Name:   "resB",
			ID:     "imported-id",
			Parent: p.NewURN("mlc:index:Component", "comp", ""),
		},
	}).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)

	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 5)

	// Ensure that the resource 3 is the component.
	comp := snap.Resources[3]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::mlc:index:Component::comp"), comp.URN)
	// Ensure it's marked as a component.
	assert.False(t, comp.Custom, "expected component resource to not be marked as custom")

	// Ensure resource 4 is the custom resource
	custom := snap.Resources[4]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::mlc:index:Component$pkgA:m:typA::resB"), custom.URN)
	// Ensure it's marked as custom.
	assert.True(t, custom.Custom, "expected custom resource to be marked as custom")
}

// Regression test for https://github.com/pulumi/pulumi-gcp/issues/1900
// Check that if a provider normalizes an input we don't display that as a diff.
func TestImportInputDiff(t *testing.T) {
	t.Parallel()

	upInputs := resource.PropertyMap{
		"foo": resource.NewStringProperty("barz"),
	}
	readInputs := resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	readOutputs := resource.PropertyMap{
		"foo":  resource.NewStringProperty("bar"),
		"frob": resource.NewNumberProperty(1),
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(plugin.GetSchemaRequest) ([]byte, error) {
					return []byte(importSchema), nil
				},
				DiffF: func(
					urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{
						Changes: plugin.DiffNone,
						StableKeys: []resource.PropertyKey{
							"foo",
						},
					}, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{
						ID:      "actual-id",
						Inputs:  readInputs,
						Outputs: readOutputs,
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: upInputs,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}
	project := p.GetProject()

	// Run an import.
	snap, err := ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resA",
		ID:   "imported-id",
	}}).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			assertDisplay(t, events, filepath.Join("testdata", "output", t.Name(), "import"))
			return err
		}, "0")
	assert.NoError(t, err)
	// 3 because Import magic's up a Stack resource.
	assert.Len(t, snap.Resources, 3)

	// Import should save the ID, inputs and outputs
	assert.Equal(t, resource.ID("actual-id"), snap.Resources[2].ID)
	assert.Equal(t, readInputs, snap.Resources[2].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[2].Outputs)

	// Run an update that changes the input but the provider normalizes it.
	snap, err = TestOp(Update).RunStep(
		project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			assertDisplay(t, events, filepath.Join("testdata", "output", t.Name(), "up"))
			return err
		}, "1")
	assert.NoError(t, err)

	// Update should have updated to the new inputs from the program.
	assert.Equal(t, resource.ID("actual-id"), snap.Resources[1].ID)
	assert.Equal(t, upInputs, snap.Resources[1].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[1].Outputs)
}
