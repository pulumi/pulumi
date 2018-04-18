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
	"github.com/pulumi/pulumi/pkg/workspace"
)

type StackReference interface {
	fmt.Stringer
	EngineName() tokens.QName
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
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error

	// Destroy destroys all of this stack's resources.
	Destroy(stackRef StackReference, proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error

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
