// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import (
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack is a stack associated with a particular backend implementation.
type Stack interface {
	Name() tokens.QName         // this stack's name.
	Config() config.Map         // the current config map.
	Snapshot() *deploy.Snapshot // the latest deployment snapshot.
	Backend() Backend           // the backend this stack belongs to.

	// Preview changes to this stack.
	Preview(proj *workspace.Project, root string,
		opts engine.UpdateOptions, displayOpts DisplayOptions) error
	// Update this stack.
	Update(proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error
	// Destroy this stack's resources.
	Destroy(proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error

	Remove(force bool) (bool, error)                                  // remove this stack.
	GetLogs(query operations.LogQuery) ([]operations.LogEntry, error) // list log entries for this stack.
	ExportDeployment() (*apitype.UntypedDeployment, error)            // export this stack's deployment.
	ImportDeployment(deployment *apitype.UntypedDeployment) error     // import the given deployment into this stack.
}

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(s Stack, force bool) (bool, error) {
	return s.Backend().RemoveStack(s.Name(), force)
}

// PreviewStack initiates a preview of the current workspace's contents.
func PreviewStack(s Stack, proj *workspace.Project, root string,
	opts engine.UpdateOptions, displayOpts DisplayOptions) error {
	return s.Backend().Preview(s.Name(), proj, root, opts, displayOpts)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(s Stack, proj *workspace.Project, root string,
	m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error {
	return s.Backend().Update(s.Name(), proj, root, m, opts, displayOpts)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(s Stack, proj *workspace.Project, root string,
	m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error {
	return s.Backend().Destroy(s.Name(), proj, root, m, opts, displayOpts)
}

// GetStackCrypter fetches the encrypter/decrypter for a stack.
func GetStackCrypter(s Stack) (config.Crypter, error) {
	return s.Backend().GetStackCrypter(s.Name())
}

// GetStackLogs fetches a list of log entries for the current stack in the current backend.
func GetStackLogs(s Stack, query operations.LogQuery) ([]operations.LogEntry, error) {
	return s.Backend().GetLogs(s.Name(), query)
}

// ExportStackDeployment exports the given stack's deployment as an opaque JSON message.
func ExportStackDeployment(s Stack) (*apitype.UntypedDeployment, error) {
	return s.Backend().ExportDeployment(s.Name())
}

// ImportStackDeployment imports the given deployment into the indicated stack.
func ImportStackDeployment(s Stack, deployment *apitype.UntypedDeployment) error {
	return s.Backend().ImportDeployment(s.Name(), deployment)
}
