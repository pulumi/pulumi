// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operations

import (
	"context"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRootStackMetadata(op display.StepOp) engine.StepEventMetadata {
	return engine.StepEventMetadata{
		Op:   op,
		URN:  resource.DefaultRootStackURN("stack", "project"),
		Type: resource.RootStackType,
		New: &engine.StepEventStateMetadata{
			URN:  resource.DefaultRootStackURN("stack", "project"),
			Type: resource.RootStackType,
		},
	}
}

type stateOptions struct {
	Parent   resource.URN
	Provider *providers.Reference
	Inputs   resource.PropertyMap
}

func makeStateMetadata(
	t *testing.T, name string, typ tokens.Type, custom bool, opts stateOptions,
) engine.StepEventStateMetadata {
	var provider string
	if opts.Provider != nil {
		provider = opts.Provider.String()
	}

	parent := opts.Parent
	if parent == "" {
		// Default to the root stack
		parent = resource.DefaultRootStackURN("stack", "project")
	}

	urn := resource.CreateURN(name, string(typ), "", "project", "stack")

	return engine.StepEventStateMetadata{
		URN:      urn,
		Type:     typ,
		Custom:   custom,
		Provider: provider,
		Parent:   parent,
		Inputs:   opts.Inputs,
	}
}

func makeMetadata(op display.StepOp, state engine.StepEventStateMetadata) engine.StepEventMetadata {
	return engine.StepEventMetadata{
		Op:       op,
		URN:      state.URN,
		Type:     state.Type,
		Provider: state.Provider,
		New:      &state,
		Res:      &state,
	}
}

// Adds a default provider for the given type if appropriate (i.e. the type token is pkg:mod:typ).
func addDefaultProvider(t *testing.T, typ tokens.Type, events chan<- engine.Event) string {
	pkg := typ.Package()
	if pkg != "" {
		state := makeStateMetadata(t, "default_1_2_3", providers.MakeProviderType(pkg), true, stateOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"version": "1.2.3",
			}),
		})
		state.ID = providers.UnknownID
		events <- engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: makeMetadata(deploy.OpCreate, state),
		})

		ref, err := providers.NewReference(state.URN, state.ID)
		require.NoError(t, err)
		return ref.String()
	}
	return ""
}

// TestBuildImportFile_SingleResource tests that for each case below creating a single resource it generates
// the expected import file.
func TestBuildImportFile_SingleResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    engine.StepEventStateMetadata
		expected importSpec
	}{
		{
			"custom",
			makeStateMetadata(t, "res", "pkg:mod:typ", true, stateOptions{}),
			importSpec{
				ID:      "<PLACEHOLDER>",
				Type:    "pkg:mod:typ",
				Name:    "res",
				Version: "1.2.3",
			},
		},
		{
			"component",
			makeStateMetadata(t, "comp", "my/component", false, stateOptions{}),
			importSpec{
				Type:      "my/component",
				Name:      "comp",
				Component: true,
			},
		},
		{
			"remote component",
			makeStateMetadata(t, "rem", "mlc:index:typ", false, stateOptions{}),
			importSpec{
				Type:      "mlc:index:typ",
				Name:      "rem",
				Component: true,
				Remote:    true,
				Version:   "1.2.3",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := make(chan engine.Event)
			importFilePromise := buildImportFile(events)

			// Always create a root stack (which shouldn't show in the import file)
			events <- engine.NewEvent(engine.ResourcePreEventPayload{
				Metadata: makeRootStackMetadata(deploy.OpCreate),
			})

			// Set the default provider (if any)
			tt.input.Provider = addDefaultProvider(t, tt.input.Type, events)

			// And then create the one resource we're testing
			events <- engine.NewEvent(engine.ResourcePreEventPayload{
				Metadata: makeMetadata(deploy.OpCreate, tt.input),
			})

			// Finally, close the events channel to signal that we're done
			close(events)
			importFile, err := importFilePromise.Result(context.Background())
			require.NoError(t, err)
			// There shouldn't be any thing in the name table
			assert.Len(t, importFile.NameTable, 0)
			// And there should be the one expected resource in the resources table
			require.Len(t, importFile.Resources, 1)
			assert.Equal(t, tt.expected, importFile.Resources[0])
		})
	}
}

// TestBuildImportFile_ExistingParent test that if we try to import a resource that has a parent that already
// exists we correctly reference that parent in the importSpec and name table.
func TestBuildImportFile_ExistingParent(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	// Pretend the root stack already exists
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeRootStackMetadata(deploy.OpSame),
	})

	provider := addDefaultProvider(t, "pkg:mod:parent", events)

	// And then same a parent resource
	parentState := makeStateMetadata(t, "parent", "pkg:mod:parent", true, stateOptions{})
	parentState.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpSame, parentState),
	})

	// And then create a resource that has that parent
	state := makeStateMetadata(t, "child", "pkg:mod:child", true, stateOptions{
		Parent: parentState.URN,
	})
	state.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, state),
	})

	// Finally, close the events channel to signal that we're done
	close(events)
	importFile, err := importFilePromise.Result(context.Background())
	require.NoError(t, err)

	// There should be one thing in the name table
	require.Len(t, importFile.NameTable, 1)
	assert.Equal(t, parentState.URN, importFile.NameTable["parent"])

	// And there should be the one expected resource in the resources table
	require.Len(t, importFile.Resources, 1)
	expected := importSpec{
		ID:      "<PLACEHOLDER>",
		Type:    "pkg:mod:child",
		Name:    "child",
		Parent:  "parent",
		Version: "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[0])
}

// TestBuildImportFile_NewParent test that if we try to import a resource that has a parent that we're
// importing with the child that we correctly reference that parent in the importSpec and don't add it to
// the name table (because the import system should auto-build the URN).
func TestBuildImportFile_NewParent(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	// Pretend the root stack already exists
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeRootStackMetadata(deploy.OpSame),
	})

	provider := addDefaultProvider(t, "pkg:mod:parent", events)

	// And then create a parent resource
	parentState := makeStateMetadata(t, "parent", "pkg:mod:parent", true, stateOptions{})
	parentState.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, parentState),
	})

	// And then create a resource that has that parent
	state := makeStateMetadata(t, "child", "pkg:mod:child", true, stateOptions{
		Parent: parentState.URN,
	})
	state.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, state),
	})

	// Finally, close the events channel to signal that we're done
	close(events)
	importFile, err := importFilePromise.Result(context.Background())
	require.NoError(t, err)

	// There shouldn't be anything in the name table
	assert.Len(t, importFile.NameTable, 0)

	// And there should be the two expected resources in the resources table
	require.Len(t, importFile.Resources, 2)
	expected := importSpec{
		ID:      "<PLACEHOLDER>",
		Type:    "pkg:mod:parent",
		Name:    "parent",
		Version: "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[0])
	expected = importSpec{
		ID:      "<PLACEHOLDER>",
		Type:    "pkg:mod:child",
		Name:    "child",
		Parent:  "parent",
		Version: "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[1])
}

// TestBuildImportFile_ExistingProvider test that if we try to import a resource that has an explicit provider
// that we reference that correctly in the name table and importSpec.
func TestBuildImportFile_ExistingProvider(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	// Pretend the root stack already exists
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeRootStackMetadata(deploy.OpSame),
	})

	// And then same a provider resource
	providerState := makeStateMetadata(t, "prov", "pulumi:providers:pkg", true, stateOptions{
		Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
			"version": "3.2.1",
		}),
	})
	uuid, err := uuid.NewV4()
	require.NoError(t, err)
	providerState.ID = resource.ID(uuid.String())
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpSame, providerState),
	})

	// And then create a resource that has that provider
	providerRef, err := providers.NewReference(providerState.URN, providerState.ID)
	require.NoError(t, err)
	state := makeStateMetadata(t, "res", "pkg:mod:typ", true, stateOptions{
		Provider: &providerRef,
	})
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, state),
	})

	// Finally, close the events channel to signal that we're done
	close(events)
	importFile, err := importFilePromise.Result(context.Background())
	require.NoError(t, err)

	// There should be one thing in the name table
	require.Len(t, importFile.NameTable, 1)
	assert.Equal(t, providerState.URN, importFile.NameTable["prov"])

	// And there should be the one expected resource in the resources table
	require.Len(t, importFile.Resources, 1)
	expected := importSpec{
		ID:       "<PLACEHOLDER>",
		Type:     "pkg:mod:typ",
		Name:     "res",
		Provider: "prov",
		Version:  "3.2.1",
	}
	assert.Equal(t, expected, importFile.Resources[0])
}

// TestBuildImportFile_NewProvider test that if we try to import a resource that has an explicit provider
// that we haven't created yet that we error. We can't handle this case yet in the import system.
func TestBuildImportFile_NewProvider(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	// Pretend the root stack already exists
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeRootStackMetadata(deploy.OpSame),
	})

	// And then create a provider resource
	providerState := makeStateMetadata(t, "prov", "pulumi:providers:pkg", true, stateOptions{})
	providerState.ID = providers.UnknownID
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, providerState),
	})

	// And then create a resource that has that provider
	providerRef, err := providers.NewReference(providerState.URN, providerState.ID)
	require.NoError(t, err)
	state := makeStateMetadata(t, "res", "pkg:mod:typ", true, stateOptions{
		Provider: &providerRef,
	})
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, state),
	})

	// Finally, close the events channel to signal that we're done
	close(events)

	// This should error because we can't yet handle importing an explicit provider
	_, err = importFilePromise.Result(context.Background())
	assert.ErrorContains(
		t, err,
		"cannot import resource \"urn:pulumi:stack::project::pkg:mod:typ::res\" "+
			"with a new explicit provider \"urn:pulumi:stack::project::pulumi:providers:pkg::prov\"")
}

// TestBuildImportFile_DuplicateNames test that if we try to import resources with the same name we add a
// suffix to their names to make them unique.
func TestBuildImportFile_DuplicateNames(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	// Pretend the root stack already exists
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeRootStackMetadata(deploy.OpSame),
	})

	// Set the default provider (if any)
	provider := addDefaultProvider(t, "pkg:i:t", events)

	// And then create one resource of one type
	stateA := makeStateMetadata(t, "res", "pkg:index:typA", true, stateOptions{})
	stateA.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, stateA),
	})

	// And then create another resource of a different type but the same name
	stateB := makeStateMetadata(t, "res", "pkg:index:typB", true, stateOptions{})
	stateB.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, stateB),
	})

	// Finally, close the events channel to signal that we're done
	close(events)

	importFile, err := importFilePromise.Result(context.Background())
	require.NoError(t, err)

	// There should be nothing in the name table
	require.Len(t, importFile.NameTable, 0)

	// And there should be the two expected resource in the resources table
	require.Len(t, importFile.Resources, 2)
	expected := importSpec{
		ID:      "<PLACEHOLDER>",
		Type:    "pkg:index:typA",
		Name:    "res",
		Version: "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[0])
	expected = importSpec{
		ID:          "<PLACEHOLDER>",
		Type:        "pkg:index:typB",
		Name:        "resTypB",
		LogicalName: "res",
		Version:     "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[1])
}

// TestBuildImportFile_NameConflict tests that if we try to import resources with the same name we add a
// suffix but ensure it doesn't conflict with any other resources original names.
func TestBuildImportFile_NameConflict(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	// Pretend the root stack already exists
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeRootStackMetadata(deploy.OpSame),
	})

	// Set the default provider (if any)
	provider := addDefaultProvider(t, "pkg:i:t", events)

	// And then create one resource of one type
	stateA := makeStateMetadata(t, "res", "pkg:index:typA", true, stateOptions{})
	stateA.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, stateA),
	})

	// And then create another resource of a different type but the same name
	stateB := makeStateMetadata(t, "res", "pkg:index:typB", true, stateOptions{})
	stateB.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, stateB),
	})

	// And then create another resource that would conflict with the default name we'd have picked for the
	// second resource.
	stateC := makeStateMetadata(t, "resTypB", "pkg:index:typB", true, stateOptions{})
	stateC.Provider = provider
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, stateC),
	})

	// Finally, close the events channel to signal that we're done
	close(events)

	importFile, err := importFilePromise.Result(context.Background())
	require.NoError(t, err)

	// There should be nothing in the name table
	require.Len(t, importFile.NameTable, 0)

	// And there should be the two expected resource in the resources table
	require.Len(t, importFile.Resources, 3)
	expected := importSpec{
		ID:      "<PLACEHOLDER>",
		Type:    "pkg:index:typA",
		Name:    "res",
		Version: "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[0])
	expected = importSpec{
		ID:          "<PLACEHOLDER>",
		Type:        "pkg:index:typB",
		Name:        "resTypB2",
		LogicalName: "res",
		Version:     "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[1])
	expected = importSpec{
		ID:      "<PLACEHOLDER>",
		Type:    "pkg:index:typB",
		Name:    "resTypB",
		Version: "1.2.3",
	}
	assert.Equal(t, expected, importFile.Resources[2])
}

// Regression test for https://github.com/pulumi/pulumi/issues/15002
// Creates two unimported resources of the same name to ensure that we correctly track only the
// taken names in the import file.
func TestBuildImportFile_regress_15002(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			URN: "urn:pulumi:test::test::pkgA:m:typA::resA",
		},
	})
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			URN: "urn:pulumi:test::test::pkgA:m:a-different-type::resA",
		},
	})
	// Finally, close the events channel to signal that we're done
	close(events)

	importFile, err := importFilePromise.Result(context.Background())
	require.NoError(t, err)
	require.Empty(t, importFile.Resources)
}

// Regression test for https://github.com/pulumi/pulumi/issues/15068
// Creates an explicit provider and a resource that uses it.
func TestBuildImportFile_regress_15068(t *testing.T) {
	t.Parallel()

	events := make(chan engine.Event)
	importFilePromise := buildImportFile(events)

	// Create the root stack.
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeRootStackMetadata(deploy.OpCreate),
	})

	// And then create a provider resource
	providerState := makeStateMetadata(t, "prov", "pulumi:providers:pkg", true, stateOptions{})
	providerState.ID = providers.UnknownID
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, providerState),
	})

	// And then create a resource that has that provider
	providerRef, err := providers.NewReference(providerState.URN, providerState.ID)
	require.NoError(t, err)
	state := makeStateMetadata(t, "res", "pkg:mod:typ", true, stateOptions{
		Provider: &providerRef,
	})
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, state),
	})

	// Try and write another event to the channel, this will block if we haven't correctly closed the channel.
	state = makeStateMetadata(t, "res2", "pkg:mod:typ", true, stateOptions{})
	events <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: makeMetadata(deploy.OpCreate, state),
	})

	// Finally, close the events channel to signal that we're done
	close(events)

	// This should error because we can't yet handle importing an explicit provider
	_, err = importFilePromise.Result(context.Background())
	assert.ErrorContains(
		t, err,
		"cannot import resource \"urn:pulumi:stack::project::pkg:mod:typ::res\" "+
			"with a new explicit provider \"urn:pulumi:stack::project::pulumi:providers:pkg::prov\"")
}
