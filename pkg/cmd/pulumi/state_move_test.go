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
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createStackWithResources(t *testing.T, b diy.Backend, stackName string, resources []*resource.State) backend.Stack {
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

func TestMoveLeafResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// urn:pulumi:stack2::pulumi-test-moves::pulumi:providers:aws::default_5_43_0::4b0fb629-cd65-4914-a34b-23a258996f7b

	providerURN := resource.NewURN("sourceStack", "test", "pulumi:providers:d", "a_provider", "1.0.0")
	sourceResources := []*resource.State{
		{
			URN:  providerURN,
			Type: "pulumi:providers:d",
			ID:   "provider_id",
		},
		{
			URN:  resource.NewURN("sourceStack", "test", "d:e:f", "a:b:c", "name"),
			Type: "a:b:c",
			Inputs: resource.PropertyMap{
				resource.PropertyKey("html"): resource.NewStringProperty("<html@tags>"),
			},
			Provider: string(providerURN),
		},
	}
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

	stateMoveCmd := stateMoveCmd{}
	stateMoveCmd.Run(ctx, sourceStack, destStack, []string{string(sourceResources[0].URN)})

	// mp := &secrets.MockProvider{}
	// mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
	// 	return b64.NewBase64SecretsManager(), nil
	// })

	// TODO: Check that the state on disk also reflects this state
	// sourceSnapshot, err := sourceStack.Snapshot(ctx, mp)
	// assert.NoError(t, err)
	// assert.Equal(t, 0, len(sourceSnapshot.Resources))

	// destSnapshot, err := destStack.Snapshot(ctx, mp)
	// assert.NoError(t, err)
	// assert.Equal(t, 1, len(destSnapshot.Resources))
}
