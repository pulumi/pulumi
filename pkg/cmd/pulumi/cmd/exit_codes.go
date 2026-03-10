package cmd

import (
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// Exit code taxonomy for the Pulumi CLI. These values form part of the
// contract for automation and agent integrations and must be treated as
// stable once released.
const (
	ExitSuccess             = 0
	ExitCodeError           = 1 // generic/unclassified error
	ExitConfigurationError  = 2 // invalid flags, config, or invocation
	ExitAuthenticationError = 3
	ExitResourceError       = 4
	ExitPolicyViolation     = 5
	ExitStackNotFound       = 6
	ExitNoChanges           = 7
	ExitCancelled           = 8
	ExitTimeout             = 9
	ExitInternalError       = 255
)

// ExitCodeFor maps an error to a process exit code, based on its concrete or
// wrapped type. This function is the single choke point for mapping errors
// into the public exit code contract.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// Respect bail semantics first – still a failure, but don't print anything.
	if result.IsBail(err) {
		return ExitCodeError
	}

	// Authentication / authorization problems.
	switch {
	case errors.As(err, &backenderr.LoginRequiredError{}),
		errors.As(err, &backenderr.ForbiddenError{}),
		errors.As(err, &backenderr.MissingEnvVarForNonInteractiveError{}):
		return ExitAuthenticationError
	}

	// Stack context problems.
	switch {
	case errors.As(err, &backenderr.StackNotFoundError{}),
		errors.As(err, &backenderr.NoStacksError{}),
		errors.As(err, &backenderr.NoStackSelectedError{}),
		errors.As(err, &backenderr.StackStateNotFoundError{}):
		return ExitStackNotFound
	}

	// User-initiated cancellation.
	if errors.As(err, &backenderr.CancelledError{}) {
		return ExitCancelled
	}

	// Expectation / invariant failures like --expect-no-changes.
	if errors.As(err, &backenderr.NoChangesExpectedError{}) {
		return ExitNoChanges
	}

	// Non-interactive mode without confirmation flags.
	if errors.As(err, &backenderr.NoConfirmationInNonInteractiveError{}) {
		return ExitConfigurationError
	}

	// TODO: Wire policy, timeout, and resource-specific errors as they gain
	// concrete types in engine/backend layers.

	// Internal/unexpected errors can be identified here in future; for now,
	// fall back to the generic error exit code.
	return ExitCodeError
}

