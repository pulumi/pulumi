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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that a journal entry whose state holds a string containing non-UTF8 bytes inside a secret
// records that fact, and that a deployment rebuilt from the journal is gated on the byteString
// feature. The secret is encrypted in the serialized entry, so replay cannot detect this on its own.
func TestJournalByteStringRoundTrip(t *testing.T) {
	t.Parallel()

	const raw = "\x00hello \x80\xfe\xff world\xf0\x28"

	state := &pkgresource.State{
		Type:    tokens.Type("pkgA:index:res"),
		URN:     resource.NewURN("dev", "proj", "", tokens.Type("pkgA:index:res"), "r1"),
		Custom:  true,
		Inputs:  property.Map{},
		Outputs: property.NewMap(map[string]property.Value{"out": property.New(raw).WithSecret(true)}),
	}

	engineEntry := engine.JournalEntry{
		Kind:        engine.JournalEntrySuccess,
		SequenceID:  1,
		OperationID: 1,
		State:       state,
	}

	serialized, err := SerializeJournalEntry(t.Context(), engineEntry, b64.NewBase64SecretsManager().Encrypter())
	require.NoError(t, err)
	assert.True(t, serialized.RequiresByteString)

	replayer := NewJournalReplayer(&apitype.DeploymentV3{})
	require.NoError(t, replayer.Add(serialized))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	assert.Contains(t, deployment.Features, "byteString")
	assert.Equal(t, apitype.DeploymentSchemaVersionLatest, deployment.Version)
}
