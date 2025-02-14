// Copyright 2024, Pulumi Corporation.
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
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // Modifies the environment
func TestNewStackSecretsManagerLoaderFromEnv(t *testing.T) {
	// Arrange.
	cases := []struct {
		envValue string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
	}

	for _, c := range cases {
		t.Setenv("PULUMI_FALLBACK_TO_STATE_SECRETS_MANAGER", c.envValue)

		// Act.
		loader := NewStackSecretsManagerLoaderFromEnv()

		// Assert.
		assert.Equal(
			t, c.expected, loader.FallbackToState,
			"FallbackToState (%v) should be pulled from the environment (%v)", c.envValue,
		)
	}
}

func TestStackSecretsManagerLoaderDecrypterFallsBack(t *testing.T) {
	t.Parallel()

	// Arrange.
	fellback := false
	sm := &secrets.MockSecretsManager{
		TypeF: func() string { return "mock" },
		DecrypterF: func() config.Decrypter {
			fellback = true
			return config.NopDecrypter
		},
	}
	snap := &deploy.Snapshot{SecretsManager: sm}

	s := &backend.MockStack{
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return snap, nil
		},
	}

	ps := &workspace.ProjectStack{}
	ssml := SecretsManagerLoader{FallbackToState: true}

	// Act.
	_, state, err := ssml.GetDecrypter(context.Background(), s, ps)

	// Assert.
	//
	// We can't assert that the decrypter we get back is the one we returned since
	// it may be decorated (e.g. with a caching decrypter). We thus assert that
	// our fallback was called as a proxy.
	assert.NoError(t, err)
	assert.Equal(
		t, SecretsManagerUnchanged, state,
		"A mock decrypter should have no effect on the project stack",
	)
	assert.True(t, fellback, "Should have fallen back to the mock decrypter")
}

func TestStackSecretsManagerLoaderDecrypterUpdatesConfig(t *testing.T) {
	t.Parallel()

	// Arrange.
	sm := &secrets.MockSecretsManager{
		TypeF:      func() string { return passphrase.Type },
		DecrypterF: func() config.Decrypter { return config.NopDecrypter },
		StateF:     func() json.RawMessage { return []byte(`{"salt":"test-salt"}`) },
	}
	snap := &deploy.Snapshot{SecretsManager: sm}

	s := &backend.MockStack{
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return snap, nil
		},
	}

	ps := &workspace.ProjectStack{}
	ssml := SecretsManagerLoader{FallbackToState: true}

	// Act.
	_, state, err := ssml.GetDecrypter(context.Background(), s, ps)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(
		t, SecretsManagerShouldSave, state,
		"A fallback passphrase decrypter should be written to the project stack",
	)
	assert.Equal(t, "test-salt", ps.EncryptionSalt, "The encryption salt should be set on the project stack")
}

func TestStackSecretsManagerLoaderDecrypterUsesDefaultSecretsManager(t *testing.T) {
	t.Parallel()

	// Arrange.
	defaulted := false
	sm := &secrets.MockSecretsManager{
		TypeF: func() string { return "mock" },
		DecrypterF: func() config.Decrypter {
			defaulted = true
			return config.NopDecrypter
		},
	}

	s := &backend.MockStack{
		DefaultSecretManagerF: func(info *workspace.ProjectStack) (secrets.Manager, error) {
			return sm, nil
		},
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{}, nil
		},
	}

	ps := &workspace.ProjectStack{}
	ssml := SecretsManagerLoader{FallbackToState: false}

	// Act.
	_, state, err := ssml.GetDecrypter(context.Background(), s, ps)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(
		t, SecretsManagerUnchanged, state,
		"No fallback manager should mean no changes to the project stack",
	)
	assert.True(t, defaulted, "Should have loaded the default decrypter")
}

func TestStackSecretsManagerLoaderEncrypterFallsBack(t *testing.T) {
	t.Parallel()

	// Arrange.
	fellback := false
	sm := &secrets.MockSecretsManager{
		TypeF: func() string { return "mock" },
		EncrypterF: func() config.Encrypter {
			fellback = true
			return config.NopEncrypter
		},
	}
	snap := &deploy.Snapshot{SecretsManager: sm}

	s := &backend.MockStack{
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return snap, nil
		},
	}

	ps := &workspace.ProjectStack{}
	ssml := SecretsManagerLoader{FallbackToState: true}

	// Act.
	_, state, err := ssml.GetEncrypter(context.Background(), s, ps)

	// Assert.
	//
	// We can't assert that the encrypter we get back is the one we returned since
	// it may be decorated (e.g. with a caching encrypter). We thus assert that
	// our fallback was called as a proxy.
	assert.NoError(t, err)
	assert.Equal(
		t, SecretsManagerUnchanged, state,
		"A mock encrypter should have no effect on the project stack",
	)
	assert.True(t, fellback, "Should have fallen back to the mock encrypter")
}

func TestStackSecretsManagerLoaderEncrypterUpdatesConfig(t *testing.T) {
	t.Parallel()

	// Arrange.
	sm := &secrets.MockSecretsManager{
		TypeF:      func() string { return passphrase.Type },
		EncrypterF: func() config.Encrypter { return config.NopEncrypter },
		StateF:     func() json.RawMessage { return []byte(`{"salt":"test-salt"}`) },
	}
	snap := &deploy.Snapshot{SecretsManager: sm}

	s := &backend.MockStack{
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return snap, nil
		},
	}

	ps := &workspace.ProjectStack{}
	ssml := SecretsManagerLoader{FallbackToState: true}

	// Act.
	_, state, err := ssml.GetEncrypter(context.Background(), s, ps)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(
		t, SecretsManagerShouldSave, state,
		"A fallback passphrase encrypter should be written to the project stack",
	)
	assert.Equal(t, "test-salt", ps.EncryptionSalt, "The encryption salt should be set on the project stack")
}

func TestStackSecretsManagerLoaderEncrypterUsesDefaultSecretsManager(t *testing.T) {
	t.Parallel()

	// Arrange.
	defaulted := false
	sm := &secrets.MockSecretsManager{
		TypeF: func() string { return "mock" },
		EncrypterF: func() config.Encrypter {
			defaulted = true
			return config.NopEncrypter
		},
	}

	s := &backend.MockStack{
		DefaultSecretManagerF: func(info *workspace.ProjectStack) (secrets.Manager, error) {
			return sm, nil
		},
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{}, nil
		},
	}

	ps := &workspace.ProjectStack{}
	ssml := SecretsManagerLoader{FallbackToState: false}

	// Act.
	_, state, err := ssml.GetEncrypter(context.Background(), s, ps)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(
		t, SecretsManagerUnchanged, state,
		"No fallback manager should mean no changes to the project stack",
	)
	assert.True(t, defaulted, "Should have loaded the default encrypter")
}
