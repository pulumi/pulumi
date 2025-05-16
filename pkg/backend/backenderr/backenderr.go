// Copyright 2019-2024, Pulumi Corporation.
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

package backenderr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

var ErrNotFound = NotFoundError{}

// ErrNoPreviousDeployment is returned when there isn't a previous deployment.
var ErrNoPreviousDeployment = errors.New("no previous deployment")

// ErrLoginRequired is returned when a command requires logging in.
var ErrLoginRequired = LoginRequiredError{}

var ErrForbidden = ForbiddenError{}

type ForbiddenError struct{ Err error }

func (err ForbiddenError) Unwrap() error {
	return err.Err
}

func (err ForbiddenError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return "forbidden"
}

func (ForbiddenError) Is(other error) bool {
	switch other.(type) {
	case ForbiddenError, *ForbiddenError,
		registry.ForbiddenError, *registry.ForbiddenError:
		return true
	default:
		return false
	}
}

type LoginRequiredError struct{}

func (LoginRequiredError) Error() string {
	return "this command requires logging in; try running `pulumi login` first"
}

func (LoginRequiredError) Is(other error) bool {
	switch other.(type) {
	case LoginRequiredError, *LoginRequiredError,
		registry.UnauthorizedError, *registry.UnauthorizedError:
		return true
	default:
		return false
	}
}

// StackAlreadyExistsError is returned from CreateStack when the stack already exists in the backend.
type StackAlreadyExistsError struct {
	StackName string
}

func (e StackAlreadyExistsError) Error() string {
	return fmt.Sprintf("stack '%v' already exists", e.StackName)
}

// OverStackLimitError is returned from CreateStack when the organization is billed per-stack and
// is over its stack limit.
type OverStackLimitError struct {
	Message string
}

func (e OverStackLimitError) Error() string {
	m := e.Message
	m = strings.ReplaceAll(m, "Conflict: ", "over stack limit: ")
	return m
}

// ConflictingUpdateError represents an error which occurred while starting an update/destroy operation.
// Another update of the same stack was in progress, so the operation got canceled due to this conflict.
type ConflictingUpdateError struct {
	Err error // The error that occurred while starting the operation.
}

func (c ConflictingUpdateError) Error() string {
	return fmt.Sprintf("%s\nTo learn more about possible reasons and resolution, visit "+
		"https://www.pulumi.com/docs/troubleshooting/#conflict", c.Err)
}

// MissingEnvVarForNonInteractiveError represents a situation where the CLI is run in
// non-interactive mode and that requires certain env vars to be set.
type MissingEnvVarForNonInteractiveError struct {
	Var env.Var
}

func (err MissingEnvVarForNonInteractiveError) Error() string {
	return err.Var.Name() + " must be set for login during non-interactive CLI sessions"
}

// NotFoundError wraps another error, indicating that the underlying problem was that a
// resource was not found.
type NotFoundError struct {
	Err error
}

func (err NotFoundError) Error() string {
	if err.Err == nil {
		return "not found"
	}
	return err.Err.Error()
}

func (err NotFoundError) Unwrap() error { return err.Err }

func (err NotFoundError) Is(other error) bool {
	switch other.(type) {
	case NotFoundError, *NotFoundError,
		// By returning true for `registry.NotFoundError`, we can return true for
		// calling code that checks:
		//
		//	errors.Is(err, registry.NotFoundError)
		registry.NotFoundError, *registry.NotFoundError:
		return true
	default:
		return false
	}
}
