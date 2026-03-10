package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func TestExitCodeFor_KnownErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: ExitSuccess},
		{name: "bail error", err: result.BailError(errors.New("bail")), want: ExitCodeError},
		{name: "login required", err: backenderr.LoginRequiredError{}, want: ExitAuthenticationError},
		{name: "forbidden", err: backenderr.ForbiddenError{}, want: ExitAuthenticationError},
		{name: "missing env var for non-interactive login", err: backenderr.MissingEnvVarForNonInteractiveError{}, want: ExitAuthenticationError},
		{name: "stack not found", err: backenderr.StackNotFoundError{StackName: "missing"}, want: ExitStackNotFound},
		{name: "no stacks", err: backenderr.NoStacksError{}, want: ExitStackNotFound},
		{name: "no stack selected", err: backenderr.NoStackSelectedError{}, want: ExitStackNotFound},
		{name: "stack state not found", err: backenderr.StackStateNotFoundError{StackName: "missing"}, want: ExitStackNotFound},
		{name: "cancelled update", err: backenderr.CancelledError{Operation: "update"}, want: ExitCancelled},
		{name: "no changes expected", err: backenderr.NoChangesExpectedError{Operation: "preview"}, want: ExitNoChanges},
		{name: "no confirmation in non-interactive", err: backenderr.NoConfirmationInNonInteractiveError{}, want: ExitConfigurationError},
		{name: "wrapped auth error", err: wrap(errors.New("outer"), backenderr.LoginRequiredError{}), want: ExitAuthenticationError},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExitCodeFor(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}

func wrap(outer error, inner error) error {
	return errors.Join(outer, inner)
}

