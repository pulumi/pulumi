// Copyright 2023-2024, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

// When a backend doesn't support the --teams flag,
// stack creation should fail.
func TestStackInit_teamsUnsupportedByBackend(t *testing.T) {
	t.Parallel()

	mockBackend := &backend.MockBackend{
		NameF: func() string {
			return "mock"
		},
		ParseStackReferenceF: func(ref string) (backend.StackReference, error) {
			return &backend.MockStackReference{}, nil
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
			assert.NotEmpty(t, opts.Teams, "expected teams to be set")
			return nil, backend.ErrTeamsNotSupported
		},
		DefaultSecretManagerF: func(*workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
	}
	cmd := &stackInitCmd{
		teams:     []string{"red", "blue"},
		stackName: "dev",
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	}

	err := cmd.Run(context.Background(), nil /* args */)
	assert.ErrorContains(t, err, "stack dev uses the mock backend: mock does not support --teams")
}

// This test demonstrates that newCreateStackOptions will filter
// out teams consisting exclusively of whitespace. NB: It's not intended
// to fully validate the correctness of team names. For example, it doesn't
// check for illegal punctuation, length, or other measures of correctness.
// To keep the codebase DRY, we pass along team names as-is to the Pulumi Cloud,
// with the exception of trimming whitespace, and allow the Pulumi Cloud to
// validate them.
func TestNewCreateStackOptsFiltersWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		giveTeams []string
		wantTeams []string
	}{
		{
			name: "empty",
			// no raw or valid teams
			giveTeams: []string{},
			wantTeams: []string{},
		},
		{
			name:      "single valid",
			giveTeams: []string{"TeamRocket"},
			wantTeams: []string{"TeamRocket"},
		},
		{
			name:      "all invalid",
			giveTeams: []string{" ", "\t", "\n"},
			wantTeams: []string{},
		},
		{
			name:      "valid and invalid",
			giveTeams: []string{" ", "Edward", "\t", "Jacob", "\n"},
			wantTeams: []string{"Edward", "Jacob"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// If the test case provides at least one valid team,
			// then the options should be non-nil.
			got := sanitizeTeams(tt.giveTeams)
			assert.ElementsMatch(t, tt.wantTeams, got)
		})
	}
}
