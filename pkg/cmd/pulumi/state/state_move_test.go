// Copyright 2016-2025, Pulumi Corporation.
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

package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createStackWithResources(
	t *testing.T, b diy.Backend, stackName string, resources []*resource.State,
) backend.Stack {
	sm := b64.NewBase64SecretsManager()

	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, resources, nil, deploy.SnapshotMetadata{})
	ctx := t.Context()

	udep, err := stack.SerializeUntypedDeployment(ctx, snap, nil /*opts*/)
	require.NoError(t, err)

	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	err = b.ImportDeployment(ctx, s, udep)
	require.NoError(t, err)

	return s
}

type MoveOptions struct {
	IncludeParents bool
}

func runMove(
	t *testing.T, sourceResources []*resource.State, args []string,
) (*deploy.Snapshot, *deploy.Snapshot, bytes.Buffer) {
	return runMoveWithOptions(t, sourceResources, args, &MoveOptions{})
}

func runMoveWithOptions(
	t *testing.T, sourceResources []*resource.State, args []string, options *MoveOptions,
) (*deploy.Snapshot, *deploy.Snapshot, bytes.Buffer) {
	return runMoveWithOptionsAndDestResources(t, sourceResources, []*resource.State{}, args, options)
}

func runMoveWithDestResources(
	t *testing.T, sourceResources, destResources []*resource.State, args []string,
) (*deploy.Snapshot, *deploy.Snapshot, bytes.Buffer) {
	return runMoveWithOptionsAndDestResources(t, sourceResources, destResources, args, &MoveOptions{})
}

func runMoveWithOptionsAndDestResources(
	t *testing.T, sourceResources, destResources []*resource.State, args []string, options *MoveOptions,
) (*deploy.Snapshot, *deploy.Snapshot, bytes.Buffer) {
	ctx := t.Context()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"
	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/test/destStack"
	destStack := createStackWithResources(t, b, destStackName, destResources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:            true,
		Stdout:         &stdout,
		Colorizer:      colors.Never,
		IncludeParents: options.IncludeParents,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, args, mp, mp)
	require.NoError(t, err)

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destSnapshot, err := destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	return sourceSnapshot, destSnapshot, stdout
}

func TestMoveLeafResource(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
	}

	sourceSnapshot, destSnapshot, stdout := runMove(t, sourceResources, []string{string(sourceResources[1].URN)})

	//nolint:lll
	expectedStdout := `Planning to move the following resources from organization/test/sourceStack to organization/test/destStack:

  - urn:pulumi:sourceStack::test::a:b:c::name

Successfully moved resources from organization/test/sourceStack to organization/test/destStack
`
	assert.Equal(t, expectedStdout, stdout.String())

	require.Len(t, sourceSnapshot.Resources, 1) // Only the provider should remain in the source stack

	require.Len(t, destSnapshot.Resources, 3) // We expect the root stack, the provider, and the moved resource
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
}

func TestChildrenAreBeingMoved(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "a:b:c", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
		},
	}

	sourceSnapshot, destSnapshot, stdout := runMove(t, sourceResources, []string{string(sourceResources[1].URN)})

	assert.Contains(t, stdout.String(),
		"Planning to move the following resources from organization/test/sourceStack to organization/test/destStack:\n\n"+
			"  - urn:pulumi:sourceStack::test::a:b:c::name\n"+
			"  - urn:pulumi:sourceStack::test::a:b:c$a:b:c::name2")

	require.Len(t, sourceSnapshot.Resources, 1) // Only the provider should remain in the source stack

	require.Len(t, destSnapshot.Resources, 4) // We expect the root stack, the provider, and the moved resources
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c$a:b:c::name2"),
		destSnapshot.Resources[3].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[3].Parent)
}

func TestMoveResourceWithDependencies(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resToMoveURN := resource.NewURN("sourceStack", "test", "", "a:b:c", "resToMove")
	remainingDepURN := resource.NewURN("sourceStack", "test", "", "a:b:c", "remainingDep")
	depsURN := resource.NewURN("sourceStack", "test", "", "a:b:c", "deps")
	deletedWithURN := resource.NewURN("sourceStack", "test", "", "a:b:c", "deletedWith")
	propDepsURN := resource.NewURN("sourceStack", "test", "", "a:b:c", "propDeps")
	movedChildURN := resource.NewURN("sourceStack", "test", "a:b:c", "a:b:c", "movedChildURN")
	dependsOnMovedChildURN := resource.NewURN("sourceStack", "test", "a:b:c", "a:b:c", "dependsOnMovedChildURN")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      remainingDepURN,
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
		{
			URN:          resToMoveURN,
			Type:         "a:b:c",
			Provider:     string(providerURN) + "::provider_id",
			Dependencies: []resource.URN{remainingDepURN},
		},
		{
			URN:      movedChildURN,
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resToMoveURN,
		},
		{
			URN:          dependsOnMovedChildURN,
			Type:         "a:b:c",
			Provider:     string(providerURN) + "::provider_id",
			Parent:       resToMoveURN,
			Dependencies: []resource.URN{movedChildURN},
		},
		{
			URN:          depsURN,
			Type:         "a:b:c",
			Provider:     string(providerURN) + "::provider_id",
			Dependencies: []resource.URN{resToMoveURN, remainingDepURN},
		},
		{
			URN:         deletedWithURN,
			Type:        "a:b:c",
			Provider:    string(providerURN) + "::provider_id",
			DeletedWith: resToMoveURN,
		},
		{
			URN:      propDepsURN,
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"key": {resToMoveURN, remainingDepURN},
			},
		},
	}

	sourceSnapshot, destSnapshot, stdout := runMove(t, sourceResources, []string{string(resToMoveURN)})

	//nolint:lll
	expectedStdout := `Planning to move the following resources from organization/test/sourceStack to organization/test/destStack:

  - urn:pulumi:sourceStack::test::a:b:c::resToMove
  - urn:pulumi:sourceStack::test::a:b:c$a:b:c::movedChildURN
  - urn:pulumi:sourceStack::test::a:b:c$a:b:c::dependsOnMovedChildURN

The following resources remaining in organization/test/sourceStack have dependencies on resources moved to organization/test/destStack:

  - urn:pulumi:sourceStack::test::a:b:c::deps has a dependency on urn:pulumi:sourceStack::test::a:b:c::resToMove
  - urn:pulumi:sourceStack::test::a:b:c::deletedWith is marked as deleted with urn:pulumi:sourceStack::test::a:b:c::resToMove
  - urn:pulumi:sourceStack::test::a:b:c::propDeps (key) has a property dependency on urn:pulumi:sourceStack::test::a:b:c::resToMove

The following resources being moved to organization/test/destStack have dependencies on resources in organization/test/sourceStack:

  - urn:pulumi:sourceStack::test::a:b:c::resToMove has a dependency on urn:pulumi:sourceStack::test::a:b:c::remainingDep

If you go ahead with moving these dependencies, it will be necessary to create the appropriate inputs and outputs in the program for the stack the resources are moved to.

Successfully moved resources from organization/test/sourceStack to organization/test/destStack
`
	assert.Equal(t, expectedStdout, stdout.String())

	// Only the provider and the resources that are not moved should remain in the source stack
	require.Len(t, sourceSnapshot.Resources, 5)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::pulumi:providers:a::default_1_0_0"),
		sourceSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c::remainingDep"),
		sourceSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c::deps"),
		sourceSnapshot.Resources[2].URN)
	require.Len(t, sourceSnapshot.Resources[2].Dependencies, 1)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c::remainingDep"),
		sourceSnapshot.Resources[2].Dependencies[0])
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c::deletedWith"),
		sourceSnapshot.Resources[3].URN)
	assert.Equal(t, urn.URN(""), sourceSnapshot.Resources[3].DeletedWith)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c::propDeps"),
		sourceSnapshot.Resources[4].URN)
	require.Len(t, sourceSnapshot.Resources[4].PropertyDependencies, 1)
	require.Len(t, sourceSnapshot.Resources[4].PropertyDependencies["key"], 1)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c::remainingDep"),
		sourceSnapshot.Resources[4].PropertyDependencies["key"][0])

	require.Len(t, destSnapshot.Resources, 5) // We expect the root stack, the provider, and the moved resources
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::resToMove"),
		destSnapshot.Resources[2].URN)
	assert.Empty(t, destSnapshot.Resources[2].Dependencies)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c$a:b:c::movedChildURN"),
		destSnapshot.Resources[3].URN)
	assert.Empty(t, destSnapshot.Resources[3].Dependencies)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c$a:b:c::dependsOnMovedChildURN"),
		destSnapshot.Resources[4].URN)
	require.Len(t, destSnapshot.Resources[4].Dependencies, 1)
}

func TestMoveWithExistingProvider(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Inputs: resource.PropertyMap{
				"key": resource.NewProperty("value"),
			},
		},
		{
			URN:      resource.NewURN("destStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
	}

	destResources := []*resource.State{
		{
			URN:    resource.NewURN("destStack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "other_provider_id",
			Custom: true,
			Inputs: resource.PropertyMap{
				"key": resource.NewProperty("different value"),
			},
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"

	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/test/destStack"
	destStack := createStackWithResources(t, b, destStackName, destResources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[1].URN)}, mp, mp)
	assert.ErrorContains(t, err, "provider urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0 "+
		"already exists in destination stack")
}

func TestMoveWithExistingResource(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
	}

	otherProviderURN := resource.NewURN("destStack", "test", "", "pulumi:providers:a", "default_1_0_1")
	destResources := []*resource.State{
		{
			URN:    otherProviderURN,
			Type:   "pulumi:providers:a::default_1_0_1",
			ID:     "other_provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("destStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(otherProviderURN) + "::other_provider_id",
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"

	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/test/destStack"
	destStack := createStackWithResources(t, b, destStackName, destResources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[1].URN)}, mp, mp)
	assert.ErrorContains(t, err, "resource urn:pulumi:destStack::test::a:b:c::name "+
		"already exists in destination stack")
}

func TestParentsAreBeingMoved(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "a:b:c", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
		},
	}

	sourceSnapshot, destSnapshot, _ := runMoveWithOptions(t, sourceResources, []string{string(sourceResources[2].URN)},
		&MoveOptions{IncludeParents: true})

	require.Len(t, sourceSnapshot.Resources, 1) // Only the provider should remain in the source stack

	require.Len(t, destSnapshot.Resources, 4) // We expect the root stack, the provider, and the moved resources
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c$a:b:c::name2"),
		destSnapshot.Resources[3].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[3].Parent)
}

func TestEmptySourceStack(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tmpDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"
	sourceRef, err := b.ParseStackReference(sourceStackName)
	require.NoError(t, err)

	sourceStack, err := b.CreateStack(ctx, sourceRef, "", nil, nil)
	require.NoError(t, err)

	destStackName := "organization/test/destStack"
	destRef, err := b.ParseStackReference(destStackName)
	require.NoError(t, err)

	destStack, err := b.CreateStack(ctx, destRef, "", nil, nil)
	require.NoError(t, err)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{"not-there"}, mp, mp)
	assert.ErrorContains(t, err, "source stack has no resources")
}

//nolint:paralleltest // changest directory for process
func TestEmptyDestStack(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
	}

	sourceStackName := "organization/test/sourceStack"
	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/test/destStack"
	destRef, err := b.ParseStackReference(destStackName)
	require.NoError(t, err)

	destStack, err := b.CreateStack(ctx, destRef, "", nil, nil)
	require.NoError(t, err)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})
	mp = mp.Add("passphrase", func(state json.RawMessage) (secrets.Manager, error) {
		return passphrase.NewPromptingPassphraseSecretsManagerFromState(state)
	})

	t.Chdir(tmpDir)

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")
	// Set up dummy project in this directory
	err = os.WriteFile("Pulumi.yaml", []byte(`
name: test
runtime: mock
`), 0o600)
	require.NoError(t, err)
	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[1].URN)}, mp, mp)
	require.NoError(t, err)

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destStack, err = b.GetStack(ctx, destRef)
	require.NoError(t, err)

	destSnapshot, err := destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	require.Len(t, sourceSnapshot.Resources, 1) // Only the provider should remain in the source stack

	require.Len(t, destSnapshot.Resources, 3) // We expect the root stack, the provider, and the moved resource
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
}

func TestMovingProvidersWithSameID(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.DefaultRootStackURN("sourceStack", "test"),
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"

	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destResources := []*resource.State{}
	destStackName := "organization/test/destStack"
	destStack := createStackWithResources(t, b, destStackName, destResources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[2].URN)}, mp, mp)
	require.NoError(t, err)

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destSnapshot, err := destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// The provider, rootstack and one resource remain
	require.Len(t, sourceSnapshot.Resources, 3)
	// The provider, rootstack and the moved resource are in the destination
	require.Len(t, destSnapshot.Resources, 3)

	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[3].URN)}, mp, mp)
	require.NoError(t, err)

	sourceSnapshot, err = sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destSnapshot, err = destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Only the provider and root stack remain
	require.Len(t, sourceSnapshot.Resources, 2)

	require.Len(t, destSnapshot.Resources, 4) // We expect the root stack, the provider, and the moved resources
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, resource.DefaultRootStackURN("destStack", "test"),
		destSnapshot.Resources[1].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name2"),
		destSnapshot.Resources[3].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[3].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[3].Parent)
}

func TestMoveUnknownResource(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "a:b:c", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"

	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destResources := []*resource.State{}
	destStackName := "organization/test/destStack"
	destStack := createStackWithResources(t, b, destStackName, destResources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{"not-a-urn"}, mp, mp)
	assert.ErrorContains(t, err, "no resources found to move")

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "warning: Resource not-a-urn not found in source stack")
	require.Len(t, sourceSnapshot.Resources, 3) // No resources should be moved
}

func TestProviderIsReparented(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.DefaultRootStackURN("sourceStack", "test"),
		},
	}

	sourceSnapshot, destSnapshot, _ := runMove(t, sourceResources, []string{string(sourceResources[2].URN)})

	// Only the provider and the root stack should remain in the source stack
	require.Len(t, sourceSnapshot.Resources, 2)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::pulumi:pulumi:Stack::test-sourceStack"),
		sourceSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::pulumi:providers:a::default_1_0_0"),
		sourceSnapshot.Resources[1].URN)

	require.Len(t, destSnapshot.Resources, 3) // We expect the root stack, the provider, and the moved resource
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, resource.DefaultRootStackURN("destStack", "test"),
		destSnapshot.Resources[1].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
}

func TestMoveProvider(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "a:b:c", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"

	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destResources := []*resource.State{}
	destStackName := "organization/test/destStack"
	destStack := createStackWithResources(t, b, destStackName, destResources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(providerURN)}, mp, mp)
	assert.ErrorContains(t, err, "cannot move provider")

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	require.Len(t, sourceSnapshot.Resources, 3) // No resources should be moved
}

func TestMoveRootStack(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.DefaultRootStackURN("sourceStack", "test"),
		},
	}

	sourceSnapshot, destSnapshot, _ := runMove(t, sourceResources, []string{string(sourceResources[0].URN)})

	// Expect the root stack and the provider to remain in the source stack
	require.Len(t, sourceSnapshot.Resources, 2)
	// All other resources are moved to the destination
	require.Len(t, destSnapshot.Resources, 3)

	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, resource.DefaultRootStackURN("destStack", "test"),
		destSnapshot.Resources[1].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
}

//nolint:paralleltest // changes directory for process
func TestMoveSecret(t *testing.T) {
	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.DefaultRootStackURN("sourceStack", "test"),
			Outputs:  resource.PropertyMap{"secret": resource.MakeSecret(resource.NewProperty("secret"))},
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"
	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/test/destStack"
	destRef, err := b.ParseStackReference(destStackName)
	require.NoError(t, err)

	destStack, err := b.CreateStack(ctx, destRef, "", nil, nil)
	require.NoError(t, err)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})
	mp = mp.Add("passphrase", func(state json.RawMessage) (secrets.Manager, error) {
		return passphrase.NewPromptingPassphraseSecretsManagerFromState(state)
	})

	t.Chdir(tmpDir)

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")
	// Set up dummy project in this directory
	err = os.WriteFile("Pulumi.yaml", []byte(`
name: test
runtime: mock
`), 0o600)
	require.NoError(t, err)

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[2].URN)}, mp, mp)
	require.NoError(t, err)

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destStack, err = b.GetStack(ctx, destRef)
	require.NoError(t, err)

	destSnapshot, err := destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Expect the root stack and the provider to remain in the source stack
	require.Len(t, sourceSnapshot.Resources, 2)
	// All other resources are moved to the destination
	require.Len(t, destSnapshot.Resources, 3)

	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, resource.DefaultRootStackURN("destStack", "test"),
		destSnapshot.Resources[1].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
	assert.True(t, destSnapshot.Resources[2].Outputs["secret"].IsSecret())
	assert.Equal(t, "secret", destSnapshot.Resources[2].Outputs["secret"].SecretValue().Element.V)
}

func TestMoveSecretOutsideOfProjectDir(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.DefaultRootStackURN("sourceStack", "test"),
			Outputs:  resource.PropertyMap{"secret": resource.MakeSecret(resource.NewProperty("secret"))},
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"
	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/test/destStack"
	destRef, err := b.ParseStackReference(destStackName)
	require.NoError(t, err)

	destStack, err := b.CreateStack(ctx, destRef, "", nil, nil)
	require.NoError(t, err)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})
	mp = mp.Add("passphrase", func(state json.RawMessage) (secrets.Manager, error) {
		return passphrase.NewPromptingPassphraseSecretsManagerFromState(state)
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}

	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[2].URN)}, mp, mp)
	//nolint:lll
	assert.ErrorContains(t, err, "destination stack has no secret manager. To move resources either initialize the stack with a secret manager, or run the pulumi state move command from the destination project directory")

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destStack, err = b.GetStack(ctx, destRef)
	require.NoError(t, err)

	destSnapshot, err := destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Expect no resources to be moved
	require.Len(t, sourceSnapshot.Resources, 3)
	assert.Nil(t, destSnapshot)
}

func TestMoveSecretNotInDestProjectDir(t *testing.T) {
	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.DefaultRootStackURN("sourceStack", "test"),
			Outputs:  resource.PropertyMap{"secret": resource.MakeSecret(resource.NewProperty("secret"))},
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"
	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/anotherproject/destStack"
	destRef, err := b.ParseStackReference(destStackName)
	require.NoError(t, err)

	destStack, err := b.CreateStack(ctx, destRef, "", nil, nil)
	require.NoError(t, err)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})
	mp = mp.Add("passphrase", func(state json.RawMessage) (secrets.Manager, error) {
		return passphrase.NewPromptingPassphraseSecretsManagerFromState(state)
	})

	t.Chdir(tmpDir)

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")
	// Set up dummy project in this directory
	err = os.WriteFile("Pulumi.yaml", []byte(`
name: yetanotherproject
runtime: mock
`), 0o600)
	require.NoError(t, err)

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}

	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[2].URN)}, mp, mp)
	//nolint:lll
	assert.ErrorContains(t, err, "destination stack has no secret manager. To move resources either initialize the stack with a secret manager, or run the pulumi state move command from the destination project directory")

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destStack, err = b.GetStack(ctx, destRef)
	require.NoError(t, err)

	destSnapshot, err := destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Expect no resources to be moved
	require.Len(t, sourceSnapshot.Resources, 3)
	assert.Nil(t, destSnapshot)
}

func TestMoveProviderWithSameInputs(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Inputs: resource.PropertyMap{
				"key": resource.NewProperty("value"),
			},
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
	}

	destProviderURN := resource.NewURN("destStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	destResources := []*resource.State{
		{
			URN:    destProviderURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "another_provider_id",
			Custom: true,
			Inputs: resource.PropertyMap{
				"key": resource.NewProperty("value"),
			},
		},
	}

	sourceSnapshot, destSnapshot, stdout := runMoveWithDestResources(
		t, sourceResources, destResources, []string{string(sourceResources[1].URN)})

	//nolint:lll
	expectedStdout := `Planning to move the following resources from organization/test/sourceStack to organization/test/destStack:

  - urn:pulumi:sourceStack::test::a:b:c::name

Successfully moved resources from organization/test/sourceStack to organization/test/destStack
`
	assert.Equal(t, expectedStdout, stdout.String())

	require.Len(t, sourceSnapshot.Resources, 1) // Only the provider should remain in the source stack

	require.Len(t, destSnapshot.Resources, 3) // We expect the root stack, the provider, and the moved resource
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::another_provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
}

// TODO: Add a test checking the error text for when the reverting to the original destination state fails.
// See https://github.com/pulumi/pulumi/pull/17208/files#diff-cbb48e4e8470d1946c5073f9d6ece05f454b340cc66ca4d9fbf7901e0a8b5c47L1330
//
//nolint:lll // The link is too long
func TestMoveLockedBackendRevertsDestination(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "a:b:c", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
		},
	}

	ctx := t.Context()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"
	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destStackName := "organization/anotherproject/destStack"
	destStack := createStackWithResources(t, b, destStackName, []*resource.State{})

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	var stdout bytes.Buffer

	stateMoveCmd := stateMoveCmd{
		Yes:       true,
		Stdout:    &stdout,
		Colorizer: colors.Never,
	}

	lockingB, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	err = lockingB.Lock(ctx, sourceStack.Ref())
	require.NoError(t, err)

	err = stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[2].URN)}, mp, mp)
	assert.ErrorContains(t, err, "None of the resources have been moved.  Please fix the error and try again")

	sourceStack, err = b.GetStack(ctx, sourceStack.Ref())
	require.NoError(t, err)
	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	destStack, err = b.GetStack(ctx, destStack.Ref())
	require.NoError(t, err)
	destSnapshot, err := destStack.Snapshot(ctx, mp)
	require.NoError(t, err)

	require.Len(t, destSnapshot.Resources, 0)

	require.Len(t, sourceSnapshot.Resources, 4)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::pulumi:pulumi:Stack::test-sourceStack"),
		sourceSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::pulumi:providers:a::default_1_0_0"),
		sourceSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c::name"),
		sourceSnapshot.Resources[2].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::a:b:c$a:b:c::name2"),
		sourceSnapshot.Resources[3].URN)
}

// Test that when a resource is moved and it has a provider as parent, the provider is still
// treated as a normal provider, and not as parent. This means its children are not moved by default.
func TestProviderParentsAreTreatedAsProviders(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	sourceResources := []*resource.State{
		{
			URN:  resource.DefaultRootStackURN("sourceStack", "test"),
			Type: "pulumi:pulumi:Stack",
		},
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Parent: resource.DefaultRootStackURN("sourceStack", "test"),
			Custom: true,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.URN(string(providerURN)),
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "pulumi:providers:a", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.URN(string(providerURN)),
		},
	}

	sourceSnapshot, destSnapshot, stdout := runMoveWithOptions(
		t, sourceResources, []string{string(sourceResources[2].URN)},
		&MoveOptions{IncludeParents: true})

	assert.Contains(t, stdout.String(),
		"Planning to move the following resources from organization/test/sourceStack to organization/test/destStack:\n\n"+
			"  - urn:pulumi:sourceStack::test::a:b:c::name")

	// The root stack, provider and "name2" should remain in the source stack
	require.Len(t, sourceSnapshot.Resources, 3)

	require.Len(t, destSnapshot.Resources, 3) // We expect the root stack, the provider, and the moved resources
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
}
