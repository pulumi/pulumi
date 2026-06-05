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

package backend

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJournalExtensionParameterizeRoundTrip(t *testing.T) {
	t.Parallel()

	ref := apitype.ExtensionRef("ref-1")
	ext := apitype.Extension{Name: "myext", Version: "1.0.0", Value: []byte("Hello")}

	engineEntry := engine.JournalEntry{
		Kind:         engine.JournalEntryExtensionParameterize,
		SequenceID:   1,
		OperationID:  1,
		ExtensionRef: &ref,
		Extension:    &ext,
	}

	serialized, err := SerializeJournalEntry(context.Background(), engineEntry, config.NopEncrypter)
	require.NoError(t, err)
	assert.Equal(t, apitype.JournalEntryKindExtensionParameterize, serialized.Kind)
	require.NotNil(t, serialized.ExtensionRef)
	require.NotNil(t, serialized.Extension)
	assert.Equal(t, ref, *serialized.ExtensionRef)
	assert.Equal(t, ext, *serialized.Extension)

	replayer := NewJournalReplayer(&apitype.DeploymentV3{})
	require.NoError(t, replayer.Add(serialized))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.NotNil(t, deployment.Deployment)
	require.Contains(t, deployment.Deployment.Extensions, ref)
	assert.Equal(t, ext, deployment.Deployment.Extensions[ref])
}

func TestJournalReplayerSeedsExtensionsFromBase(t *testing.T) {
	t.Parallel()

	ref := apitype.ExtensionRef("base-ref")
	ext := apitype.Extension{Name: "base-ext", Version: "1.0.0", Value: []byte("baseline")}

	base := &apitype.DeploymentV3{
		Extensions: map[apitype.ExtensionRef]apitype.Extension{ref: ext},
	}
	replayer := NewJournalReplayer(base)

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.Contains(t, deployment.Deployment.Extensions, ref,
		"extensions from base must survive replay even with no extension journal entries")
	assert.Equal(t, ext, deployment.Deployment.Extensions[ref])
}
