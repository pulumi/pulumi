package backenderr

import backenderr "github.com/pulumi/pulumi/sdk/v3/pkg/backend/backenderr"

// StackAlreadyExistsError is returned from CreateStack when the stack already exists in the backend.
type StackAlreadyExistsError = backenderr.StackAlreadyExistsError

// OverStackLimitError is returned from CreateStack when the organization is billed per-stack and
// is over its stack limit.
type OverStackLimitError = backenderr.OverStackLimitError

// ConflictingUpdateError represents an error which occurred while starting an update/destroy operation.
// Another update of the same stack was in progress, so the operation got canceled due to this conflict.
type ConflictingUpdateError = backenderr.ConflictingUpdateError

// MissingEnvVarForNonInteractiveError represents a situation where the CLI is run in
// non-interactive mode and that requires certain env vars to be set.
type MissingEnvVarForNonInteractiveError = backenderr.MissingEnvVarForNonInteractiveError

// NotFoundError wraps another error, indicating that the underlying problem was that a
// resource was not found.
type NotFoundError = backenderr.NotFoundError

type ForbiddenError = backenderr.ForbiddenError

type LoginRequiredError = backenderr.LoginRequiredError

var ErrNotFound = backenderr.ErrNotFound

var ErrNoPreviousDeployment = backenderr.ErrNoPreviousDeployment

var ErrLoginRequired = backenderr.ErrLoginRequired

var ErrForbidden = backenderr.ErrForbidden

