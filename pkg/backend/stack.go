// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package backend

import (
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
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

	Remove(force bool) (bool, error)                                  // remove this stack.
	Preview(debug bool, opts engine.PreviewOptions) error             // preview changes to this stack.
	Update(debug bool, opts engine.DeployOptions) error               // update this stack.
	Destroy(debug bool, opts engine.DestroyOptions) error             // destroy this stack's resources.
	GetLogs(query operations.LogQuery) ([]operations.LogEntry, error) // list log entries for this stack.
}

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(s Stack, force bool) (bool, error) {
	return s.Backend().RemoveStack(s.Name(), force)
}

// PreviewStack initiates a preview of the current workspace's contents.
func PreviewStack(s Stack, debug bool, opts engine.PreviewOptions) error {
	return s.Backend().Preview(s.Name(), debug, opts)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(s Stack, debug bool, opts engine.DeployOptions) error {
	return s.Backend().Update(s.Name(), debug, opts)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(s Stack, debug bool, opts engine.DestroyOptions) error {
	return s.Backend().Destroy(s.Name(), debug, opts)
}

// GetStackCrypter fetches the encrypter/decrypter for a stack.
func GetStackCrypter(s Stack) (config.Crypter, error) {
	return s.Backend().GetStackCrypter(s.Name())
}

// GetStackLogs fetches a list of log entries for the current stack in the current backend.
func GetStackLogs(s Stack, query operations.LogQuery) ([]operations.LogEntry, error) {
	return s.Backend().GetLogs(s.Name(), query)
}
