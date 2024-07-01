// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"context"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
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

	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, resources, nil)
	ctx := context.Background()

	sdep, err := stack.SerializeDeployment(ctx, snap, false /* showSecrets */)
	assert.NoError(t, err)

	data, err := encoding.JSON.Marshal(sdep)
	assert.NoError(t, err)

	udep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(data),
	}

	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil)
	require.NoError(t, err)

	err = b.ImportDeployment(ctx, s, udep)
	require.NoError(t, err)

	return s
}

func runMove(t *testing.T, sourceResources []*resource.State, args []string) (*deploy.Snapshot, *deploy.Snapshot) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	sourceStackName := "organization/test/sourceStack"

	sourceStack := createStackWithResources(t, b, sourceStackName, sourceResources)

	destResources := []*resource.State{}
	destStackName := "organization/test/destStack"
	destStack := createStackWithResources(t, b, destStackName, destResources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	stateMoveCmd := stateMoveCmd{}
	err = stateMoveCmd.Run(ctx, sourceStack, destStack, args, mp)
	assert.NoError(t, err)

	sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	assert.NoError(t, err)

	destSnapshot, err := destStack.Snapshot(ctx, mp)
	assert.NoError(t, err)

	return sourceSnapshot, destSnapshot
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
			URN:      resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
	}

	sourceSnapshot, destSnapshot := runMove(t, sourceResources, []string{string(sourceResources[1].URN)})

	assert.Equal(t, 1, len(sourceSnapshot.Resources)) // Only the provider should remain in the source stack

	assert.Equal(t, 3, len(destSnapshot.Resources)) // We expect the root stack, the provider, and the moved resource
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::d:e:f$a:b:c::name"),
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
			URN:      resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "name"),
		},
	}

	sourceSnapshot, destSnapshot := runMove(t, sourceResources, []string{string(sourceResources[1].URN)})

	assert.Equal(t, 1, len(sourceSnapshot.Resources)) // Only the provider should remain in the source stack

	assert.Equal(t, 4, len(destSnapshot.Resources)) // We expect the root stack, the provider, and the moved resources
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::d:e:f$a:b:c::name"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, "urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0::provider_id",
		destSnapshot.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[2].Parent)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::d:e:f$a:b:c::name2"),
		destSnapshot.Resources[3].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::d:e:f$a:b:c::name"),
		destSnapshot.Resources[3].Parent)
}

func TestMoveResourceWithDependencies(t *testing.T) {
	t.Parallel()

	providerURN := resource.NewURN("sourceStack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resToMoveURN := resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "resToMove")
	remainingDepURN := resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "remainingDep")
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
			URN:          resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "deps"),
			Type:         "a:b:c",
			Provider:     string(providerURN) + "::provider_id",
			Dependencies: []resource.URN{resToMoveURN, remainingDepURN},
		},
		{
			URN:         resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "deletedWith"),
			Type:        "a:b:c",
			Provider:    string(providerURN) + "::provider_id",
			DeletedWith: resToMoveURN,
		},
		{
			URN:      resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "propDeps"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"key": {resToMoveURN, remainingDepURN},
			},
		},
	}

	sourceSnapshot, destSnapshot := runMove(t, sourceResources, []string{string(resToMoveURN)})
	// Only the provider and the resources that are not moved should remain in the source stack
	assert.Equal(t, 5, len(sourceSnapshot.Resources))
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::pulumi:providers:a::default_1_0_0"),
		sourceSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::d:e:f$a:b:c::remainingDep"),
		sourceSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::d:e:f$a:b:c::deps"),
		sourceSnapshot.Resources[2].URN)
	assert.Equal(t, 1, len(sourceSnapshot.Resources[2].Dependencies))
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::d:e:f$a:b:c::remainingDep"),
		sourceSnapshot.Resources[2].Dependencies[0])
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::d:e:f$a:b:c::deletedWith"),
		sourceSnapshot.Resources[3].URN)
	assert.Equal(t, urn.URN(""), sourceSnapshot.Resources[3].DeletedWith)
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::d:e:f$a:b:c::propDeps"),
		sourceSnapshot.Resources[4].URN)
	assert.Equal(t, 1, len(sourceSnapshot.Resources[4].PropertyDependencies))
	assert.Equal(t, 1, len(sourceSnapshot.Resources[4].PropertyDependencies["key"]))
	assert.Equal(t, urn.URN("urn:pulumi:sourceStack::test::d:e:f$a:b:c::remainingDep"),
		sourceSnapshot.Resources[4].PropertyDependencies["key"][0])

	assert.Equal(t, 3, len(destSnapshot.Resources)) // We expect the root stack, the provider, and the moved resource
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:pulumi:Stack::test-destStack"),
		destSnapshot.Resources[0].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::pulumi:providers:a::default_1_0_0"),
		destSnapshot.Resources[1].URN)
	assert.Equal(t, urn.URN("urn:pulumi:destStack::test::d:e:f$a:b:c::resToMove"),
		destSnapshot.Resources[2].URN)
	assert.Equal(t, 0, len(destSnapshot.Resources[2].Dependencies))
}
