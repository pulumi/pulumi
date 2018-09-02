// Copyright 2016-2018, Pulumi Corporation.
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

// Package backend encapsulates all extensibility points required to fully implement a new cloud provider.
package backend

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/workspace"
)

var (
	// ErrNoPreviousDeployment is returned when there isn't a previous deployment.
	ErrNoPreviousDeployment = errors.New("no previous deployment")
)

// StackAlreadyExistsError is returned from CreateStack when the stack already exists in the backend.
type StackAlreadyExistsError struct {
	StackName string
}

func (e StackAlreadyExistsError) Error() string {
	return fmt.Sprintf("stack '%v' already exists", e.StackName)
}

// StackReference is an opaque type that refers to a stack managed by a backend.  The CLI uses the ParseStackReference
// method to turn a string like "my-great-stack" or "pulumi/my-great-stack" into a stack reference that can be used to
// interact with the stack via the backend. Stack references are specific to a given backend and different back ends
// may interpret the string passed to ParseStackReference differently
type StackReference interface {
	// fmt.Stringer's String() method returns a string of the stack identity, suitable for display in the CLI
	fmt.Stringer
	// StackName is the name that will be passed to the Pulumi engine when preforming operations on this stack. This
	// name may not uniquely identify the stack (e.g. the cloud backend embeds owner information in the StackReference
	// but that informaion is not part of the StackName() we pass to the engine.
	StackName() tokens.QName
}

// Backend is an interface that represents actions the engine will interact with to manage stacks of cloud resources.
// It can be implemented any number of ways to provide pluggable backend implementations of the Pulumi Cloud.
type Backend interface {
	// Name returns a friendly name for this backend.
	Name() string

	// ParseStackReference takes a string representation and parses it to a reference which may be used for other
	// methods in this backend.
	ParseStackReference(s string) (StackReference, error)

	// GetStack returns a stack object tied to this backend with the given name, or nil if it cannot be found.
	GetStack(ctx context.Context, stackRef StackReference) (Stack, error)
	// CreateStack creates a new stack with the given name and options that are specific to the backend provider.
	CreateStack(ctx context.Context, stackRef StackReference, opts interface{}) (Stack, error)
	// RemoveStack removes a stack with the given name.  If force is true, the stack will be removed even if it
	// still contains resources.  Otherwise, if the stack contains resources, a non-nil error is returned, and the
	// first boolean return value will be set to true.
	RemoveStack(ctx context.Context, stackRef StackReference, force bool) (bool, error)
	// ListStacks returns a list of stack summaries for all known stacks in the target backend.
	ListStacks(ctx context.Context, projectFilter *tokens.PackageName) ([]Stack, error)

	// GetStackCrypter returns an encrypter/decrypter for the given stack's secret config values.
	GetStackCrypter(stackRef StackReference) (config.Crypter, error)

	// Preview shows what would be updated given the current workspace's contents.
	Preview(ctx context.Context, stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts UpdateOptions, scopes CancellationScopeSource) (engine.ResourceChanges, error)
	// Update updates the target stack with the current workspace's contents (config and code).
	Update(ctx context.Context, stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts UpdateOptions, scopes CancellationScopeSource) (engine.ResourceChanges, error)
	// Refresh refreshes the stack's state from the cloud provider.
	Refresh(ctx context.Context, stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts UpdateOptions, scopes CancellationScopeSource) (engine.ResourceChanges, error)
	// Destroy destroys all of this stack's resources.
	Destroy(ctx context.Context, stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts UpdateOptions, scopes CancellationScopeSource) (engine.ResourceChanges, error)

	// GetHistory returns all updates for the stack. The returned UpdateInfo slice will be in
	// descending order (newest first).
	GetHistory(ctx context.Context, stackRef StackReference) ([]UpdateInfo, error)
	// GetLogs fetches a list of log entries for the given stack, with optional filtering/querying.
	GetLogs(ctx context.Context, stackRef StackReference, query operations.LogQuery) ([]operations.LogEntry, error)
	// Get the configuration from the most recent deployment of the stack.
	GetLatestConfiguration(ctx context.Context, stackRef StackReference) (config.Map, error)

	// ExportDeployment exports the deployment for the given stack as an opaque JSON message.
	ExportDeployment(ctx context.Context, stackRef StackReference) (*apitype.UntypedDeployment, error)
	// ImportDeployment imports the given deployment into the indicated stack.
	ImportDeployment(ctx context.Context, stackRef StackReference, deployment *apitype.UntypedDeployment) error
	// Logout logs you out of the backend and removes any stored credentials.
	Logout() error
	// Returns the identity of the current user for the backend.
	CurrentUser() (string, error)
}

// UpdateOptions is the full set of update options, including backend and engine options.
type UpdateOptions struct {
	// Engine contains all of the engine-specific options.
	Engine engine.UpdateOptions
	// Display contains all of the backend display options.
	Display DisplayOptions

	// AutoApprove, when true, will automatically approve previews.
	AutoApprove bool
	// SkipPreview, when true, causes the preview step to be skipped.
	SkipPreview bool
}

// CancellationScope provides a scoped source of cancellation and termination requests.
type CancellationScope interface {
	// Context returns the cancellation context used to observe cancellation and termination requests for this scope.
	Context() *cancel.Context
	// Close closes the cancellation scope.
	Close()
}

// CancellationScopeSource provides a source for cancellation scopes.
type CancellationScopeSource interface {
	// NewScope creates a new cancellation scope.
	NewScope(events chan<- engine.Event, isPreview bool) CancellationScope
}

// tracingOptionsKey is the value used as the context key for TracingOptions.
var tracingOptionsKey struct{}

// TracingOptions describes the set of options available for configuring tracing on a per-request basis.
type TracingOptions struct {
	// PropagateSpans indicates that spans should be propagated from the client to the Pulumi service when making API
	// calls.
	PropagateSpans bool
	// IncludeTracingHeader indicates that API calls should include the indicated tracing header contents.
	TracingHeader string
}

// ContextWithTracingOptions returns a new context.Context with the indicated tracing options.
func ContextWithTracingOptions(ctx context.Context, opts TracingOptions) context.Context {
	return context.WithValue(ctx, tracingOptionsKey, opts)
}

// TracingOptionsFromContext retrieves any tracing options present in the given context. If no options are present,
// this function returns the zero value.
func TracingOptionsFromContext(ctx context.Context) TracingOptions {
	opts, _ := ctx.Value(tracingOptionsKey).(TracingOptions)
	return opts
}
