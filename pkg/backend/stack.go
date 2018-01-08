// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package backend

import (
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Stack is a stack associated with a particular backend implementation.
type Stack interface {
	Name() tokens.QName         // this stack's name.
	Config() config.Map         // the current config map.
	Snapshot() *deploy.Snapshot // the latest deployment snapshot.
	Backend() Backend           // the backend this stack belongs to.

	// preview changes to this stack.
	Preview(pkg *pack.Package, root string, debug bool, opts engine.PreviewOptions) error
	// update this stack.
	Update(pkg *pack.Package, root string, debug bool, opts engine.DeployOptions) error
	// destroy this stack's resources.
	Destroy(pkg *pack.Package, root string, debug bool, opts engine.DestroyOptions) error

	Remove(force bool) (bool, error)                                  // remove this stack.
	GetLogs(query operations.LogQuery) ([]operations.LogEntry, error) // list log entries for this stack.
	ExportDeployment() (json.RawMessage, error)                       // export this stack's deployment.
	ImportDeployment(json.RawMessage) error                           // import the given deployment into this stack.
}

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(s Stack, force bool) (bool, error) {
	return s.Backend().RemoveStack(s.Name(), force)
}

// PreviewStack initiates a preview of the current workspace's contents.
func PreviewStack(s Stack, pkg *pack.Package, root string, debug bool, opts engine.PreviewOptions) error {
	return s.Backend().Preview(s.Name(), pkg, root, debug, opts)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(s Stack, pkg *pack.Package, root string, debug bool, opts engine.DeployOptions) error {
	return s.Backend().Update(s.Name(), pkg, root, debug, opts)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(s Stack, pkg *pack.Package, root string, debug bool, opts engine.DestroyOptions) error {
	return s.Backend().Destroy(s.Name(), pkg, root, debug, opts)
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
func ExportStackDeployment(s Stack) (json.RawMessage, error) {
	return s.Backend().ExportDeployment(s.Name())
}

// ImportStackDeployment imports the given deployment into the indicated stack.
func ImportStackDeployment(s Stack, deployment json.RawMessage) error {
	return s.Backend().ImportDeployment(s.Name(), deployment)
}
