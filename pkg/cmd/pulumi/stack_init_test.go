package main

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
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
		SupportsTeamsF: func() bool {
			return false
		},
	}
	cmd := &stackInitCmd{
		teams:     []string{"red", "blue"},
		stackName: "dev",
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return mockBackend, nil
		},
	}

	err := cmd.Run(context.Background(), nil /* args */)
	assert.ErrorContains(t, err, "stack dev uses the mock backend: mock does not support --teams")
}

func TestValidateCreateStackOptsErrors(t *testing.T) {
	t.Parallel()

	// First, we create a mock backend that doesn't support teams.
	stackName := "dev"
	teams := []string{"red", "blue"}
	backendName := "mock"
	mockBackend := &backend.MockBackend{
		NameF: func() string {
			return backendName
		},
		SupportsTeamsF: func() bool {
			return false
		},
	}
	// Then, we expect validation to fail, since we provide
	// teams when they're not supported.
	_, err := validateCreateStackOpts(stackName, mockBackend, teams)
	assert.Error(t, err)
}

// This test demonstrates that validateCreateStack will filter
// out teams consisting exclusively of whitespace. NB: It's not intended
// to fully validate the correctness of team names. For example, it doesn't
// check for illegal punctuation, length, or other measures of correctness.
// To keep the codebase DRY, we pass along team names as-is to the Service,
// with the exception of trimming whitespace, and allow the Service to
// validate them.
func TestValidateCreateStackOptsFiltersWhitespace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                      string
		inputTeams, expectedTeams []string
	}{
		{
			name: "Input Is Empty",
			// no raw or valid teams
			inputTeams:    []string{},
			expectedTeams: []string{},
		},
		{
			name:          "a single valid team is provided",
			inputTeams:    []string{"TeamRocket"},
			expectedTeams: []string{"TeamRocket"},
		},
		{
			name:          "only invalid teams are provided",
			inputTeams:    []string{" ", "\t", "\n"},
			expectedTeams: []string{},
		},
		{
			name:          "mixed valid and invalid teams are provided",
			inputTeams:    []string{" ", "Edward", "\t", "Jacob", "\n"},
			expectedTeams: []string{"Edward", "Jacob"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stackName := "dev"
			mockBackend := &backend.MockBackend{
				SupportsTeamsF: func() bool {
					return true
				},
			}
			// If the test case provides at least one valid team,
			// then the options should be non-nil.
			observed, err := validateCreateStackOpts(stackName, mockBackend, tc.inputTeams)
			assert.Nil(t, err)
			assert.NotNil(t, observed)
			teams := observed.Teams
			assert.ElementsMatch(t, teams, tc.expectedTeams)
		})
	}
}
