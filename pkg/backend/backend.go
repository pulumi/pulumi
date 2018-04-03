// Copyright 2017-2018, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Backend is an interface that represents actions the engine will interact with to manage stacks of cloud resources.
// It can be implemented any number of ways to provide pluggable backend implementations of the Pulumi Cloud.
type Backend interface {
	// Name returns a friendly name for this backend.
	Name() string

	// GetStack returns a stack object tied to this backend with the given name, or nil if it cannot be found.
	GetStack(name tokens.QName) (Stack, error)
	// CreateStack creates a new stack with the given name and options that are specific to the backend provider.
	CreateStack(name tokens.QName, opts interface{}) (Stack, error)
	// RemoveStack removes a stack with the given name.  If force is true, the stack will be removed even if it
	// still contains resources.  Otherwise, if the stack contains resources, a non-nil error is returned, and the
	// first boolean return value will be set to true.
	RemoveStack(name tokens.QName, force bool) (bool, error)
	// ListStacks returns a list of stack summaries for all known stacks in the target backend.
	ListStacks() ([]Stack, error)

	// GetStackCrypter returns an encrypter/decrypter for the given stack's secret config values.
	GetStackCrypter(stack tokens.QName) (config.Crypter, error)

	// Preview initiates a preview of the current workspace's contents.
	Preview(stackName tokens.QName, proj *workspace.Project, root string,
		debug bool, opts engine.UpdateOptions, displayOpts DisplayOptions) error
	// Update updates the target stack with the current workspace's contents (config and code).
	Update(stackName tokens.QName, proj *workspace.Project, root string,
		debug bool, m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error
	// Destroy destroys all of this stack's resources.
	Destroy(stackName tokens.QName, proj *workspace.Project, root string,
		debug bool, m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error

	// GetHistory returns all updates for the stack. The returned UpdateInfo slice will be in
	// descending order (newest first).
	GetHistory(stackName tokens.QName) ([]UpdateInfo, error)
	// GetLogs fetches a list of log entries for the given stack, with optional filtering/querying.
	GetLogs(stackName tokens.QName, query operations.LogQuery) ([]operations.LogEntry, error)

	// ExportDeployment exports the deployment for the given stack as an opaque JSON message.
	ExportDeployment(stackName tokens.QName) (*apitype.UntypedDeployment, error)
	// ImportDeployment imports the given deployment into the indicated stack.
	ImportDeployment(stackName tokens.QName, deployment *apitype.UntypedDeployment) error
}
