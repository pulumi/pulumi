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
type Stack struct {
	Name      tokens.QName     // this stack's name.
	Config    config.Map       // the current config map.
	Snapshot  *deploy.Snapshot // the latest deployment snapshot.
	Backend   Backend          // the backend this stack belongs to.
	CloudURL  string           // the Pulumi.com URL, if any.
	OrgName   string           // the org name (if a cloud stack).
	CloudName string           // the PPC name (if a cloud stack).
}

// Remove returns the stack, or returns an error if it cannot.
func (s *Stack) Remove(force bool) (bool, error) {
	return s.Backend.RemoveStack(s.Name, force)
}

// Preview initiates a preview of the current workspace's contents.
func (s *Stack) Preview(debug bool, opts engine.PreviewOptions) error {
	return s.Backend.Preview(s.Name, debug, opts)
}

// Update updates the target stack with the current workspace's contents (config and code).
func (s *Stack) Update(debug bool, opts engine.DeployOptions) error {
	return s.Backend.Update(s.Name, debug, opts)
}

// Destroy destroys all of this stack's resources.
func (s *Stack) Destroy(debug bool, opts engine.DestroyOptions) error {
	return s.Backend.Destroy(s.Name, debug, opts)
}

// GetLogs fetches a list of log entries for the current stack in the current backend.
func (s *Stack) GetLogs(query operations.LogQuery) ([]operations.LogEntry, error) {
	return s.Backend.GetLogs(s.Name, query)
}
