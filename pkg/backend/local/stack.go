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

package local

import (
	"context"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack is a local stack.  This simply adds some local-specific properties atop the standard backend stack interface.
type Stack interface {
	backend.Stack
	Path() string // a path to the stack's checkpoint file on disk.
}

// localStack is a local stack descriptor.
type localStack struct {
	name     backend.StackReference // the stack's name.
	path     string                 // a path to the stack's checkpoint file on disk.
	config   config.Map             // the stack's config bag.
	snapshot *deploy.Snapshot       // a snapshot representing the latest deployment state.
	b        *localBackend          // a pointer to the backend this stack belongs to.
}

func newStack(name backend.StackReference, path string, config config.Map,
	snapshot *deploy.Snapshot, b *localBackend) Stack {
	return &localStack{
		name:     name,
		path:     path,
		config:   config,
		snapshot: snapshot,
		b:        b,
	}
}

func (s *localStack) Name() backend.StackReference                           { return s.name }
func (s *localStack) Config() config.Map                                     { return s.config }
func (s *localStack) Snapshot(ctx context.Context) (*deploy.Snapshot, error) { return s.snapshot, nil }
func (s *localStack) Backend() backend.Backend                               { return s.b }
func (s *localStack) Path() string                                           { return s.path }

func (s *localStack) Remove(ctx context.Context, force bool) (bool, error) {
	return backend.RemoveStack(ctx, s, force)
}

func (s *localStack) Preview(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.PreviewStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *localStack) Update(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.UpdateStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *localStack) Refresh(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.RefreshStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *localStack) Destroy(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.DestroyStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *localStack) GetLogs(ctx context.Context, query operations.LogQuery) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(ctx, s, query)
}

func (s *localStack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(ctx, s)
}

func (s *localStack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(ctx, s, deployment)
}
