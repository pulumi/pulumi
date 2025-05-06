// Copyright 2024, Pulumi Corporation.
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

package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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
		tt := tt
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
	assert.NoError(t, err)

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
			assert.NoError(t, err)
			return nil, nil
		},
		DefaultSecretManagerF: func(*workspace.ProjectStack) (secrets.Manager, error) {
			return expectedSm, nil
		},
	}

	stackRef := &backend.MockStackReference{}

	// Act.
	//nolint:errcheck
	CreateStack(
		context.Background(),
		pkgWorkspace.Instance,
		mockBackend,
		stackRef,
		"",    /*root*/
		nil,   /*opts*/
		false, /*setCurrent*/
		"",    /*secretsProvider*/
		false, /* useEscEnv */
	)

	// Assert.
	assert.Equal(t, expectedSm.State(), actualDeployment.SecretsProviders.State)
}
