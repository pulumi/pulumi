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

package httpstate

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Stack is a cloud stack.  This simply adds some cloud-specific properties atop the standard backend stack interface.
type Stack interface {
	backend.Stack
	OrgName() string                            // the organization that owns this stack.
	CurrentOperation() *apitype.OperationStatus // in progress operation, if applicable.
	StackIdentifier() client.StackIdentifier
}

type cloudBackendReference struct {
	name    tokens.Name
	project tokens.Name
	owner   string
	b       *cloudBackend
}

func (c cloudBackendReference) String() string {
	// When stringifying backend references, we take the current project (if present) into account.
	currentProject := c.b.currentProject

	// If the project names match, we can elide them.
	if currentProject != nil && c.project == tokens.Name(currentProject.Name) {

		// Elide owner too, if it is the default owner.
		defaultOrg, err := workspace.GetBackendConfigDefaultOrg(currentProject)
		if err == nil && defaultOrg != "" {
			// The default owner is the org
			if c.owner == defaultOrg {
				return string(c.name)
			}
		} else {
			currentUser, _, _, userErr := c.b.CurrentUser()
			if userErr == nil && c.owner == currentUser {
				return string(c.name)
			}
		}
		return fmt.Sprintf("%s/%s", c.owner, c.name)
	}

	return fmt.Sprintf("%s/%s/%s", c.owner, c.project, c.name)
}

func (c cloudBackendReference) Name() tokens.Name {
	return c.name
}

func (c cloudBackendReference) Project() (tokens.Name, bool) {
	return c.project, true
}

func (c cloudBackendReference) FullyQualifiedName() tokens.QName {
	return tokens.IntoQName(fmt.Sprintf("%v/%v/%v", c.owner, c.project, c.name.String()))
}

// cloudStack is a cloud stack descriptor.
type cloudStack struct {
	// ref is the stack's unique name.
	ref cloudBackendReference
	// orgName is the organization that owns this stack.
	orgName string
	// currentOperation contains information about any current operation being performed on the stack, as applicable.
	currentOperation *apitype.OperationStatus
	// snapshot contains the latest deployment state, allocated on first use. It's valid for the snapshot
	// itself to be nil.
	snapshot atomic.Pointer[*deploy.Snapshot]
	// b is a pointer to the backend that this stack belongs to.
	b *cloudBackend
	// tags contains metadata tags describing additional, extensible properties about this stack.
	tags map[apitype.StackTagName]string
}

func newStack(apistack apitype.Stack, b *cloudBackend) Stack {
	// Now assemble all the pieces into a stack structure.
	return &cloudStack{
		ref: cloudBackendReference{
			owner:   apistack.OrgName,
			project: tokens.Name(apistack.ProjectName),
			name:    tokens.Name(apistack.StackName.String()),
			b:       b,
		},
		orgName:          apistack.OrgName,
		currentOperation: apistack.CurrentOperation,
		tags:             apistack.Tags,
		b:                b,
		// We explicitly allocate the snapshot on first use, since it is expensive to compute.
	}
}
func (s *cloudStack) Ref() backend.StackReference                { return s.ref }
func (s *cloudStack) Backend() backend.Backend                   { return s.b }
func (s *cloudStack) OrgName() string                            { return s.orgName }
func (s *cloudStack) CurrentOperation() *apitype.OperationStatus { return s.currentOperation }
func (s *cloudStack) Tags() map[apitype.StackTagName]string      { return s.tags }

func (s *cloudStack) StackIdentifier() client.StackIdentifier {
	si, err := s.b.getCloudStackIdentifier(s.ref)
	// the above only fails when ref is of the wrong type.
	contract.AssertNoErrorf(err, "unexpected stack reference type: %T", s.ref)
	return si
}

func (s *cloudStack) Snapshot(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
	if v := s.snapshot.Load(); v != nil {
		return *v, nil
	}

	snap, err := s.b.getSnapshot(ctx, secretsProvider, s.ref)
	if err != nil {
		return nil, err
	}

	s.snapshot.Store(&snap)
	return snap, nil
}

func (s *cloudStack) Remove(ctx context.Context, force bool) (bool, error) {
	return backend.RemoveStack(ctx, s, force)
}

func (s *cloudStack) Rename(ctx context.Context, newName tokens.QName) (backend.StackReference, error) {
	return backend.RenameStack(ctx, s, newName)
}

func (s *cloudStack) Preview(
	ctx context.Context,
	op backend.UpdateOperation,
) (*deploy.Plan, sdkDisplay.ResourceChanges, result.Result) {
	return backend.PreviewStack(ctx, s, op)
}

func (s *cloudStack) Update(ctx context.Context, op backend.UpdateOperation) (sdkDisplay.ResourceChanges,
	result.Result,
) {
	return backend.UpdateStack(ctx, s, op)
}

func (s *cloudStack) Import(ctx context.Context, op backend.UpdateOperation,
	imports []deploy.Import,
) (sdkDisplay.ResourceChanges, result.Result) {
	return backend.ImportStack(ctx, s, op, imports)
}

func (s *cloudStack) Refresh(ctx context.Context, op backend.UpdateOperation) (sdkDisplay.ResourceChanges,
	result.Result,
) {
	return backend.RefreshStack(ctx, s, op)
}

func (s *cloudStack) Destroy(ctx context.Context, op backend.UpdateOperation) (sdkDisplay.ResourceChanges,
	result.Result,
) {
	return backend.DestroyStack(ctx, s, op)
}

func (s *cloudStack) Watch(ctx context.Context, op backend.UpdateOperation, paths []string) result.Result {
	return backend.WatchStack(ctx, s, op, paths)
}

func (s *cloudStack) GetLogs(ctx context.Context, secretsProvider secrets.Provider, cfg backend.StackConfiguration,
	query operations.LogQuery,
) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(ctx, secretsProvider, s, cfg, query)
}

func (s *cloudStack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(ctx, s)
}

func (s *cloudStack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(ctx, s, deployment)
}

func (s *cloudStack) DefaultSecretManager(info *workspace.ProjectStack) (secrets.Manager, error) {
	return service.NewServiceSecretsManager(s.b.Client(), s.StackIdentifier(), info)
}

// cloudStackSummary implements the backend.StackSummary interface, by wrapping
// an apitype.StackSummary struct.
type cloudStackSummary struct {
	summary apitype.StackSummary
	b       *cloudBackend
}

func (css cloudStackSummary) Name() backend.StackReference {
	contract.Assertf(css.summary.ProjectName != "", "project name must not be empty")

	return cloudBackendReference{
		owner:   css.summary.OrgName,
		project: tokens.Name(css.summary.ProjectName),
		name:    tokens.Name(css.summary.StackName),
		b:       css.b,
	}
}

func (css cloudStackSummary) LastUpdate() *time.Time {
	if css.summary.LastUpdate == nil {
		return nil
	}
	t := time.Unix(*css.summary.LastUpdate, 0)
	return &t
}

func (css cloudStackSummary) ResourceCount() *int {
	return css.summary.ResourceCount
}
