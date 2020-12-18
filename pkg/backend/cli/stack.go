// Copyright 2016-2020, Pulumi Corporation.
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

package cli

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/operations"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
)

// Stack represents a Pulumi stack.
type Stack struct {
	// id is the stack's unique identifier.
	id backend.StackIdentifier
	// cloudURL is the URl to the cloud containing this stack.
	cloudURL string
	// orgName is the organization that owns this stack.
	orgName string
	// currentOperation contains information about any current operation being performed on the stack, as applicable.
	currentOperation *apitype.OperationStatus
	// snapshot contains the latest deployment state, allocated on first use.
	snapshot **deploy.Snapshot
	// b is a pointer to the backend that this stack belongs to.
	b *Backend
	// tags contains metadata tags describing additional, extensible properties about this stack.
	tags map[apitype.StackTagName]string
}

func newStack(apistack apitype.Stack, b *Backend) *Stack {
	// Now assemble all the pieces into a stack structure.
	return &Stack{
		id: backend.StackIdentifier{
			Owner:   apistack.OrgName,
			Project: apistack.ProjectName,
			Stack:   string(apistack.StackName),
		},
		cloudURL:         b.URL(),
		orgName:          apistack.OrgName,
		currentOperation: apistack.CurrentOperation,
		snapshot:         nil, // We explicitly allocate the snapshot on first use, since it is expensive to compute.
		tags:             apistack.Tags,
		b:                b,
	}
}

func (s *Stack) ID() backend.StackIdentifier                { return s.id }
func (s *Stack) Backend() *Backend                          { return s.b }
func (s *Stack) CloudURL() string                           { return s.cloudURL }
func (s *Stack) OrgName() string                            { return s.orgName }
func (s *Stack) CurrentOperation() *apitype.OperationStatus { return s.currentOperation }
func (s *Stack) Tags() map[apitype.StackTagName]string      { return s.tags }

func (s *Stack) FriendlyName() string {
	return s.b.StackFriendlyName(s.id)
}

func (s *Stack) Snapshot(ctx context.Context) (*deploy.Snapshot, error) {
	if s.snapshot != nil {
		return *s.snapshot, nil
	}

	snap, err := s.b.getSnapshot(ctx, s.id)
	if err != nil {
		return nil, err
	}

	s.snapshot = &snap
	return *s.snapshot, nil
}

func (s *Stack) Remove(ctx context.Context, force bool) (bool, error) {
	return s.b.RemoveStack(ctx, s, force)
}

func (s *Stack) Rename(ctx context.Context, newID string) (backend.StackIdentifier, error) {
	return s.b.RenameStack(ctx, s, newID)
}

func (s *Stack) Preview(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	return s.b.Preview(ctx, s, op)
}

func (s *Stack) Update(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	return s.b.Update(ctx, s, op)
}

func (s *Stack) Import(ctx context.Context, op UpdateOperation,
	imports []deploy.Import) (engine.ResourceChanges, result.Result) {
	return s.b.Import(ctx, s, op, imports)
}

func (s *Stack) Refresh(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	return s.b.Refresh(ctx, s, op)
}

func (s *Stack) Destroy(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	return s.b.Destroy(ctx, s, op)
}

func (s *Stack) Watch(ctx context.Context, op UpdateOperation) result.Result {
	return s.b.Watch(ctx, s, op)
}

func (s *Stack) GetLogs(ctx context.Context, cfg StackConfiguration,
	query operations.LogQuery) ([]operations.LogEntry, error) {
	return s.b.GetLogs(ctx, s, cfg, query)
}

func (s *Stack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	return s.b.ExportDeployment(ctx, s)
}

func (s *Stack) ExportDeploymentForVersion(ctx context.Context, version string) (*apitype.UntypedDeployment, error) {
	return s.b.ExportDeploymentForVersion(ctx, s, version)
}

func (s *Stack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	return s.b.ImportDeployment(ctx, s, deployment)
}

func (s *Stack) ConsoleURL() (string, error) {
	return s.b.StackConsoleURL(s.id)
}
