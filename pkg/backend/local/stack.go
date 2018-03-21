// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack is a local stack.  This simply adds some local-specific properties atop the standard backend stack interface.
type Stack interface {
	backend.Stack
	Path() string // a path to the stack's checkpoint file on disk.
}

// localStack is a local stack descriptor.
type localStack struct {
	name     tokens.QName     // the stack's name.
	path     string           // a path to the stack's checkpoint file on disk.
	config   config.Map       // the stack's config bag.
	snapshot *deploy.Snapshot // a snapshot representing the latest deployment state.
	b        *localBackend    // a pointer to the backend this stack belongs to.
}

func newStack(name tokens.QName, path string, config config.Map,
	snapshot *deploy.Snapshot, b *localBackend) Stack {
	return &localStack{
		name:     name,
		path:     path,
		config:   config,
		snapshot: snapshot,
		b:        b,
	}
}

func (s *localStack) Name() tokens.QName         { return s.name }
func (s *localStack) Config() config.Map         { return s.config }
func (s *localStack) Snapshot() *deploy.Snapshot { return s.snapshot }
func (s *localStack) Backend() backend.Backend   { return s.b }
func (s *localStack) Path() string               { return s.path }

func (s *localStack) Remove(force bool) (bool, error) {
	return backend.RemoveStack(s, force)
}

func (s *localStack) Preview(proj *workspace.Project, root string,
	debug bool, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {
	return backend.PreviewStack(s, proj, root, debug, opts, displayOpts)
}

func (s *localStack) Update(proj *workspace.Project, root string,
	debug bool, m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {
	return backend.UpdateStack(s, proj, root, debug, m, opts, displayOpts)
}

func (s *localStack) Destroy(proj *workspace.Project, root string,
	debug bool, m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {
	return backend.DestroyStack(s, proj, root, debug, m, opts, displayOpts)
}

func (s *localStack) GetLogs(query operations.LogQuery) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(s, query)
}

func (s *localStack) ExportDeployment() (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(s)
}

func (s *localStack) ImportDeployment(deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(s, deployment)
}
