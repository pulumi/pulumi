// Copyright 2019, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/registry"
	pulumienv "github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

var (
	ErrNotFound NotFoundError
	// ErrNoPreviousDeployment is returned when there isn't a previous deployment.
	ErrNoPreviousDeployment = errors.New("no previous deployment")
	// ErrLoginRequired is returned when a command requires logging in.
	ErrLoginRequired LoginRequiredError
	ErrForbidden     ForbiddenError
	// ErrNonInteractiveRequiresYes is returned when a command is run
	// non-interactively without --yes.
	ErrNonInteractiveRequiresYes NonInteractiveRequiresYesError
)

// StackNotFoundError is returned when a named stack cannot be found in the backend.
type StackNotFoundError struct {
	StackName string
}

func (e StackNotFoundError) Error() string {
	if e.StackName == "" {
		return "stack not found"
	}
	return fmt.Sprintf("no stack named '%s' found", e.StackName)
}

// NoStacksError is returned when an operation requires at least one stack in a backend,
// but none are present.
type NoStacksError struct{}

func (NoStacksError) Error() string {
	return "this command requires a stack, but there are none"
}

// NoStackSelectedError is returned when a stack is required but none has been specified
// or selected in a non-interactive context.
type NoStackSelectedError struct{}

func (NoStackSelectedError) Error() string {
	return "no stack selected; specify a stack name with --stack"
}

// StackStateNotFoundError is returned when a stack exists but its state / snapshot
// cannot be located.
type StackStateNotFoundError struct {
	StackName string
}

func (e StackStateNotFoundError) Error() string {
	if e.StackName == "" {
		return "failed to find the stack snapshot. Are you in a stack?"
	}
	return fmt.Sprintf("failed to find the stack snapshot for stack %s. Are you in a stack?", e.StackName)
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
		"https://www.pulumi.com/docs/support/troubleshooting/common-issues/update-conflicts/", c.Err)
}

// MissingEnvVarForNonInteractiveError represents a situation where the CLI is run in
// non-interactive mode and that requires certain env vars to be set.
//
// It renders the same text as LoginRequiredError. The types stay separate because callers branch on them.
type MissingEnvVarForNonInteractiveError struct {
	Var env.Var
	LoginHelp
}

func (err MissingEnvVarForNonInteractiveError) Error() string {
	return loginRequiredMessage(err.Var.Name(), err.LoginHelp)
}

// LoginHelp is what a login-required error knows about the cloud the user needs to log in to. The zero value
// gives the steps that are true for any cloud.
type LoginHelp struct {
	// TokensURL, if set, is the console page where the user can create an access token. It's empty when the CLI
	// can't work out the console's address from the API's, and then `pulumi login` can't sign in through a
	// browser either: it asks for a token instead.
	TokensURL string
	// PulumiCloud is true for pulumi.com, where anyone can create a free account.
	PulumiCloud bool
}

// LoginRequiredSuggestions returns the next steps for someone who needs to log in to Pulumi Cloud, one per line.
//
// People, scripts and agents all see the same steps. Each line stands on its own, because agents often keep
// only the last few lines of a command's output.
func LoginRequiredSuggestions(help LoginHelp) []string {
	return loginRequiredSuggestions(pulumienv.AccessToken.Var().Name(), help)
}

func loginRequiredSuggestions(tokenVar string, help LoginHelp) []string {
	var lines []string
	if help.PulumiCloud {
		lines = append(lines, "Run `pulumi login` to sign in or create a free account.")
	} else {
		lines = append(lines, "Run `pulumi login` to sign in.")
	}
	if help.TokensURL != "" {
		lines = append(lines, "It opens your web browser and waits until you've signed in there.")
	}
	tokenLine := "To log in without a browser (CI, headless), set " + tokenVar + "."
	if help.TokensURL != "" {
		tokenLine += " Create a token at " + help.TokensURL + "."
	}
	return append(lines, tokenLine)
}

// LoginRequiredMessage returns the text of LoginRequiredError, for code that reports a missing login without
// returning that type.
func LoginRequiredMessage(help LoginHelp) string {
	return loginRequiredMessage(pulumienv.AccessToken.Var().Name(), help)
}

// loginRequiredMessage is the text of both LoginRequiredError and MissingEnvVarForNonInteractiveError.
func loginRequiredMessage(tokenVar string, help LoginHelp) string {
	return "Pulumi Cloud login required\n  " + strings.Join(loginRequiredSuggestions(tokenVar, help), "\n  ")
}

// NoConfirmationInNonInteractiveError represents a situation where the CLI is run
// non-interactively and no confirmation flag (such as --yes or --skip-preview) was
// provided for a destructive or mutating operation.
type NoConfirmationInNonInteractiveError struct{}

func (NoConfirmationInNonInteractiveError) Error() string {
	return "--yes or --skip-preview or --preview-only must be passed in to proceed when running in non-interactive mode"
}

// NonInteractiveRequiresYesError represents a non-interactive execution that
// requires a --yes flag (or equivalent) in order to proceed.
type NonInteractiveRequiresYesError struct{}

func (NonInteractiveRequiresYesError) Error() string {
	return "non-interactive mode requires --yes flag"
}

// NonInteractiveInputRequiredError represents a non-interactive execution where
// a required piece of input (such as a template, runtime option, or config
// value) was not provided on the command line.
type NonInteractiveInputRequiredError struct {
	Detail string
}

func (e NonInteractiveInputRequiredError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return "required input must be specified when running in non-interactive mode"
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

// DefaultOrgError indicates that an operation failed because the organization taken from
// the default organization setting does not exist or cannot be accessed by the current user.
type DefaultOrgError struct {
	Org string
	Err error
}

func (err DefaultOrgError) Error() string {
	return fmt.Sprintf("the organization %q set as the default organization does not exist or you do not have "+
		"access to it; use `pulumi org set-default` to change the default organization: %v", err.Org, err.Err)
}

func (err DefaultOrgError) Unwrap() error { return err.Err }

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

// LoginRequiredError is returned when a command requires authentication
// but the user is not logged in or their session has expired.
type LoginRequiredError struct {
	// ReauthURL, if set, is the URL the user should visit to re-authenticate.
	ReauthURL string
	LoginHelp
}

func (err LoginRequiredError) Error() string {
	if err.ReauthURL != "" {
		return fmt.Sprintf("SAML SSO authentication is required. Log in at:\n\n    %s\n", err.ReauthURL)
	}
	return LoginRequiredMessage(err.LoginHelp)
}

// CancelledError represents a user-initiated cancellation of an operation
// such as an update or destroy.
type CancelledError struct {
	Operation string
}

func (e CancelledError) Error() string {
	if e.Operation == "" {
		return "operation cancelled"
	}
	return e.Operation + " cancelled"
}

// NoChangesExpectedError represents a failure of an operation that was run
// with an expectation that no changes would occur (e.g. --expect-no-changes).
type NoChangesExpectedError struct {
	Operation string
}

func (e NoChangesExpectedError) Error() string {
	if e.Operation == "" {
		return "no changes were expected but changes were proposed"
	}
	return fmt.Sprintf("no changes were expected for %s but changes were proposed", e.Operation)
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

// IsAuthError reports whether err represents an authentication or
// authorization failure (login required, forbidden, or a missing env var
// required for non-interactive auth).
func IsAuthError(err error) bool {
	_, isLoginRequired := errors.AsType[LoginRequiredError](err)
	_, isForbidden := errors.AsType[ForbiddenError](err)
	_, isMissingEnvVar := errors.AsType[MissingEnvVarForNonInteractiveError](err)
	return isLoginRequired || isForbidden || isMissingEnvVar
}
