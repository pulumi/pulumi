// Copyright 2016, Pulumi Corporation.
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

package cmd

import (
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
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

	// `pulumi cloud api` carries its own semantic exit code on the error so
	// the taxonomy (ExitCodeError / ExitAuthenticationError / ExitCancelled
	// / etc.) reaches the shell instead of collapsing to the generic bail
	// default. Checked before the bail branch because processCmdErrors
	// wraps these in a BailError to suppress double-printing the message.
	var apiErr *cloud.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ExitCode
	}

	// Respect bail semantics first – still a failure, but don't print anything.
	if result.IsBail(err) {
		return ExitCodeError
	}

	// Authentication / authorization problems.
	if backenderr.IsAuthError(err) {
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
