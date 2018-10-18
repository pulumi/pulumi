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

package filestate

import (
	"context"
	"time"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
)

// Stack is a file stack.  This simply adds some file-specific properties atop the standard backend stack interface.
type Stack interface {
	backend.Stack
	Path() string // a path to the stack's checkpoint file on disk.
}

// fileStack is a file stack descriptor.
type fileStack struct {
	ref      backend.StackReference // the stack's reference (qualified name).
	path     string                 // a path to the stack's checkpoint file on disk.
	config   config.Map             // the stack's config bag.
	snapshot *deploy.Snapshot       // a snapshot representing the latest deployment state.
	b        *fileBackend           // a pointer to the backend this stack belongs to.
}

func newStack(ref backend.StackReference, path string, config config.Map,
	snapshot *deploy.Snapshot, b *fileBackend) Stack {
	return &fileStack{
		ref:      ref,
		path:     path,
		config:   config,
		snapshot: snapshot,
		b:        b,
	}
}

func (s *fileStack) Ref() backend.StackReference                            { return s.ref }
func (s *fileStack) Config() config.Map                                     { return s.config }
func (s *fileStack) Snapshot(ctx context.Context) (*deploy.Snapshot, error) { return s.snapshot, nil }
func (s *fileStack) Backend() backend.Backend                               { return s.b }
func (s *fileStack) Path() string                                           { return s.path }

func (s *fileStack) Remove(ctx context.Context, force bool) (bool, error) {
	return backend.RemoveStack(ctx, s, force)
}

func (s *fileStack) Preview(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.PreviewStack(ctx, s, op)
}

func (s *fileStack) Update(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.UpdateStack(ctx, s, op)
}

func (s *fileStack) Refresh(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.RefreshStack(ctx, s, op)
}

func (s *fileStack) Destroy(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.DestroyStack(ctx, s, op)
}

func (s *fileStack) GetLogs(ctx context.Context, query operations.LogQuery) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(ctx, s, query)
}

func (s *fileStack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(ctx, s)
}

func (s *fileStack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(ctx, s, deployment)
}

type fileStackSummary struct {
	s *fileStack
}

func newFileStackSummary(s *fileStack) fileStackSummary {
	return fileStackSummary{s}
}

func (lss fileStackSummary) Name() backend.StackReference {
	return lss.s.Ref()
}

func (lss fileStackSummary) LastUpdate() *time.Time {
	snap := lss.s.snapshot
	if snap != nil {
		if t := snap.Manifest.Time; !t.IsZero() {
			return &t
		}
	}
	return nil
}

func (lss fileStackSummary) ResourceCount() *int {
	snap := lss.s.snapshot
	if snap != nil {
		count := len(snap.Resources)
		return &count
	}
	return nil
}
