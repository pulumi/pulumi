// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

// Package backend encapsulates all extensibility points required to fully implement a new cloud provider.
package backend

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/workspace"
)

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
	ParseStackReference(s string, opts interface{}) (StackReference, error)

	// GetStack returns a stack object tied to this backend with the given name, or nil if it cannot be found.
	GetStack(stackRef StackReference) (Stack, error)
	// CreateStack creates a new stack with the given name and options that are specific to the backend provider.
	CreateStack(stackRef StackReference, opts interface{}) (Stack, error)
	// RemoveStack removes a stack with the given name.  If force is true, the stack will be removed even if it
	// still contains resources.  Otherwise, if the stack contains resources, a non-nil error is returned, and the
	// first boolean return value will be set to true.
	RemoveStack(stackRef StackReference, force bool) (bool, error)
	// ListStacks returns a list of stack summaries for all known stacks in the target backend.
	ListStacks(projectFilter *tokens.PackageName) ([]Stack, error)

	// GetStackCrypter returns an encrypter/decrypter for the given stack's secret config values.
	GetStackCrypter(stackRef StackReference) (config.Crypter, error)

	// Update updates the target stack with the current workspace's contents (config and code).
	Update(stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions, scopes CancellationScopeSource) error
	// Refresh refreshes the stack's state from the cloud provider.
	Refresh(stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions, scopes CancellationScopeSource) error
	// Destroy destroys all of this stack's resources.
	Destroy(stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions, scopes CancellationScopeSource) error

	// GetHistory returns all updates for the stack. The returned UpdateInfo slice will be in
	// descending order (newest first).
	GetHistory(stackRef StackReference) ([]UpdateInfo, error)
	// GetLogs fetches a list of log entries for the given stack, with optional filtering/querying.
	GetLogs(stackRef StackReference, query operations.LogQuery) ([]operations.LogEntry, error)

	// ExportDeployment exports the deployment for the given stack as an opaque JSON message.
	ExportDeployment(stackRef StackReference) (*apitype.UntypedDeployment, error)
	// ImportDeployment imports the given deployment into the indicated stack.
	ImportDeployment(stackRef StackReference, deployment *apitype.UntypedDeployment) error
	// Logout logs you out of the backend and removes any stored credentials.
	Logout() error
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
