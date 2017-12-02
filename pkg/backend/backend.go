// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package backend encapsulates all extensibility points required to fully implement a new cloud provider.
package backend

import (
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Backend is an interface that represents actions the engine will interact with to manage stacks of cloud resources.
// It can be implemented any number of ways to provide pluggable backend implementations of the Pulumi Cloud.
type Backend interface {
	// Name returns a friendly name for this backend.
	Name() string
	// IsCloud returns true if this backend is a Pulumi Cloud backend.
	IsCloud() bool

	// GetStack returns a stack object tied to this backend with the given name, or nil if it cannot be found.
	GetStack(name tokens.QName) (*Stack, error)
	// CreateStack creates a new stack with the given name and options that are specific to the backend provider.
	CreateStack(name tokens.QName, opts StackCreateOptions) error
	// RemoveStack removes a stack with the given name.  If force is true, the stack will be removed even if it
	// still contains resources.  Otherwise, if the stack contains resources, a non-nil error is returned, and the
	// first boolean return value will be set to true.
	RemoveStack(name tokens.QName, force bool) (bool, error)
	// ListStacks returns a list of stack summaries for all known stacks in the target backend.
	ListStacks() ([]*Stack, error)

	// Preview initiates a preview of the current workspace's contents.
	Preview(stackName tokens.QName, debug bool, opts engine.PreviewOptions) error
	// Update updates the target stack with the current workspace's contents (config and code).
	Update(stackName tokens.QName, debug bool, opts engine.DeployOptions) error
	// Destroy destroys all of this stack's resources.
	Destroy(stackName tokens.QName, debug bool, opts engine.DestroyOptions) error

	// GetLogs fetches a list of log entries for the given stack, with optional filtering/querying.
	GetLogs(stackName tokens.QName, query operations.LogQuery) ([]operations.LogEntry, error)
}

// StackCreateOptions are a set of options that may be passed when creating a stack.
type StackCreateOptions struct {
	CloudName string // an optional cloud name in the target backend.
}
