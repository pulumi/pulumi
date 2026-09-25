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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestStackLoadOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give       LoadOption
		offerNew   bool
		setCurrent bool
	}{
		{LoadOnly, false, false},
		{OfferNew, true, false},
		{SetCurrent, false, true},
		{OfferNew | SetCurrent, true, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.give), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t,
				tt.offerNew, tt.give.OfferNew(),
				"OfferNew did not match")
			assert.Equal(t,
				tt.setCurrent, tt.give.SetCurrent(),
				"SetCurrent did not match")
		})
	}
}

// Tests that CreateStack will send an appropriate initial state when it is asked to create a stack with a non-default
// secrets manager.
func TestCreateStack_InitialisesStateWithSecretsManager(t *testing.T) {
	t.Parallel()

	// Arrange.
	_, expectedSm, err := passphrase.NewPassphraseSecretsManager("test-passphrase")
	require.NoError(t, err)

	var actualDeployment apitype.DeploymentV3

	mockBackend := &backend.MockBackend{
		NameF: func() string {
			return "mock"
		},
		ValidateStackNameF: func(name string) error {
			assert.Equal(t, "dev", name, "stack name mismatch")
			return nil
		},
		CreateStackF: func(
			ctx context.Context,
			ref backend.StackReference,
			projectRoot string,
			initialState *apitype.UntypedDeployment,
			opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			err := json.Unmarshal(initialState.Deployment, &actualDeployment)
			require.NoError(t, err)
			return &backend.MockStack{RefF: func() backend.StackReference { return ref }}, nil
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return expectedSm, nil
		},
	}

	stackRef := &backend.MockStackReference{StringV: "dev"}

	// Act.
	//nolint:errcheck
	CreateStack(
		t.Context(),
		cmdutil.Diag(),
		pkgWorkspace.Instance,
		mockBackend,
		stackRef,
		"", /*root*/
		CreateStackOptions{},
	)

	// Assert.
	assert.Equal(t, expectedSm.State(), actualDeployment.SecretsProviders.State)
}

// Tests that CreateStack announces the new stack unless Quiet is set.
func TestCreateStackQuiet(t *testing.T) {
	t.Parallel()

	for _, quiet := range []bool{false, true} {
		t.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			t.Parallel()

			// Arrange.
			var buf bytes.Buffer
			sink := diag.DefaultSink(&buf, io.Discard, diag.FormatOptions{Color: colors.Never})

			mockBackend := &backend.MockBackend{
				NameF: func() string {
					return "mock"
				},
				CreateStackF: func(
					ctx context.Context,
					ref backend.StackReference,
					projectRoot string,
					initialState *apitype.UntypedDeployment,
					opts *backend.CreateStackOptions,
				) (backend.Stack, error) {
					return &backend.MockStack{RefF: func() backend.StackReference { return ref }}, nil
				},
				DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
					return nil, nil
				},
			}

			// Act.
			_, err := CreateStack(
				t.Context(),
				sink,
				pkgWorkspace.Instance,
				mockBackend,
				&backend.MockStackReference{StringV: "quietstack"},
				"", /*root*/
				CreateStackOptions{Quiet: quiet},
			)

			// Assert.
			require.NoError(t, err)
			if quiet {
				assert.NotContains(t, buf.String(), "Created stack")
			} else {
				assert.Contains(t, buf.String(), "Created stack 'quietstack'")
			}
		})
	}
}

// A source stack whose configuration lives in an ESC environment has nothing in its `config:` block, so
// copying it must refuse rather than silently produce a stack with no configuration source at all.
func TestCopyEntireConfigMapRefusesMainEnvironmentSource(t *testing.T) {
	t.Parallel()

	sourceStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{StringV: "acme/payments/dev"}
		},
	}
	sourceProjectStack := &workspace.ProjectStack{
		MainEnvironment: &workspace.MainEnvironment{Project: "payments", Name: "dev"},
	}

	// A nil loader proves nothing beyond the guard runs: any decrypter or encrypter lookup would panic.
	requiresSaving, err := CopyEntireConfigMap(
		t.Context(), SecretsManagerLoader{}, sourceStack, sourceProjectStack,
		&backend.MockStack{}, &workspace.ProjectStack{},
	)
	assert.False(t, requiresSaving)
	assert.ErrorContains(t, err,
		"copying configuration from stack acme/payments/dev is not supported yet: "+
			"it sets 'mainEnvironment: payments/dev'")
	assert.ErrorContains(t, err, "--esc-config")
}
