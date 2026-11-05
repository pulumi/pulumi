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

package stack

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestNewStackSecretsManager(t *testing.T) {
	t.Parallel()

	prepared := &NewStackSecrets{
		settings: &workspace.ProjectStack{EncryptionSalt: "prepared-salt"},
		sm:       &secrets.MockSecretsManager{},
	}
	fromBackend := &secrets.MockSecretsManager{}
	b := &backend.MockBackend{
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return fromBackend, nil
		},
	}

	t.Run("uses the prepared manager for a stack file without secrets settings", func(t *testing.T) {
		t.Parallel()

		ps := &workspace.ProjectStack{}
		needsSave, sm, err := newStackSecretsManager(t.Context(), b, ps, "default", prepared)
		require.NoError(t, err)
		assert.Same(t, prepared.sm, sm)
		assert.Equal(t, "prepared-salt", ps.EncryptionSalt)
		assert.True(t, needsSave)
	})

	t.Run("keeps the stack file's own secrets settings", func(t *testing.T) {
		t.Parallel()

		ps := &workspace.ProjectStack{EncryptionSalt: "file-salt"}
		_, sm, err := newStackSecretsManager(t.Context(), b, ps, "default", prepared)
		require.NoError(t, err)
		assert.Same(t, fromBackend, sm)
		assert.Equal(t, "file-salt", ps.EncryptionSalt)
	})
}
