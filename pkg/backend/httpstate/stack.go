// Copyright 2016-2023, Pulumi Corporation.
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
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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
	name    tokens.StackName
	project tokens.Name
	owner   string
	b       *cloudBackend

	// defaultOrg is the user's default organization, either configured by the user or determined otherwise.
	// If unset, will assume that there is no default organization configured and fall back to referencing
	// the user's individual org.
	defaultOrg string
}

func (c cloudBackendReference) String() string {
	// If the user has asked us to fully qualify names, we won't elide any
	// information.
	if cmdutil.FullyQualifyStackNames {
		return fmt.Sprintf("%s/%s/%s", c.owner, c.project, c.name)
	}

	// When stringifying backend references, we take the current project (if present) into account.
	currentProject := c.b.currentProject

	// If the project names match, we can elide them.
	if currentProject != nil && c.project == tokens.Name(currentProject.Name) {
		// Elide owner too, if it is the default owner.
		if c.defaultOrg != "" {
			// The default owner is the org
			if c.owner == c.defaultOrg {
				return c.name.String()
			}
		} else {
			currentUser, _, _, userErr := c.b.CurrentUser()
			if userErr == nil && c.owner == currentUser {
				return c.name.String()
			}
		}
		return fmt.Sprintf("%s/%s", c.owner, c.name)
	}

	return fmt.Sprintf("%s/%s/%s", c.owner, c.project, c.name)
}

func (c cloudBackendReference) Name() tokens.StackName {
	return c.name
}

func (c cloudBackendReference) Project() (tokens.Name, bool) {
	return c.project, true
}

func (c cloudBackendReference) Organization() (string, bool) {
	return c.owner, true
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
	// escConfigEnv caches if we expect this stack to have its config stored in an ESC environment.
	escConfigEnv *string
}

func newStack(ctx context.Context, apistack apitype.Stack, b *cloudBackend) (Stack, error) {
	stackName, err := tokens.ParseStackName(apistack.StackName.String())
	contract.AssertNoErrorf(err, "unexpected invalid stack name: %v", apistack.StackName)

	defaultOrg, err := backend.GetDefaultOrg(ctx, b, b.currentProject)
	if err != nil {
		return &cloudStack{}, fmt.Errorf("unable to lookup default org: %w", err)
	}

	var escConfigEnv *string
	if apistack.Config != nil {
		escConfigEnv = &apistack.Config.Environment
	}

	// Now assemble all the pieces into a stack structure.
	return &cloudStack{
		ref: cloudBackendReference{
			owner:      apistack.OrgName,
			project:    tokens.Name(apistack.ProjectName),
			defaultOrg: defaultOrg,
			name:       stackName,
			b:          b,
		},
		orgName:          apistack.OrgName,
		currentOperation: apistack.CurrentOperation,
		tags:             apistack.Tags,
		b:                b,
		escConfigEnv:     escConfigEnv,
	}, nil
}
func (s *cloudStack) Ref() backend.StackReference { return s.ref }

// ConfigLocation returns the ESC environment of the stack config if applicable.
func (s *cloudStack) ConfigLocation() backend.StackConfigLocation {
	return backend.StackConfigLocation{
		IsRemote: s.escConfigEnv != nil,
		EscEnv:   s.escConfigEnv,
	}
}

func (s *cloudStack) LoadRemoteConfig(ctx context.Context, project *workspace.Project,
) (*workspace.ProjectStack, error) {
	stackID, err := s.b.getCloudStackIdentifier(s.ref)
	if err != nil {
		return nil, err
	}
	stack, err := s.b.client.GetStack(ctx, stackID)
	if err != nil {
		return nil, err
	}
	if stack.Config == nil {
		return nil, nil
	}
	projectStack := &workspace.ProjectStack{
		Environment:     workspace.NewEnvironment([]string{stack.Config.Environment}),
		SecretsProvider: stack.Config.SecretsProvider,
		EncryptedKey:    stack.Config.EncryptedKey,
		EncryptionSalt:  stack.Config.EncryptionSalt,
		Config:          config.Map{},
	}
	return projectStack, nil
}

func (s *cloudStack) SaveRemoteConfig(ctx context.Context, projectStack *workspace.ProjectStack) error {
	if projectStack.Config != nil {
		// TODO: https://github.com/pulumi/pulumi/issues/19557
		return errors.New("cannot set config for a stack with cloud config")
	}
	imports := projectStack.Environment.Imports()
	if len(imports) != 1 {
		return errors.New("cloud stacks must have exactly 1 import")
	}
	stackID, err := s.b.getCloudStackIdentifier(s.ref)
	if err != nil {
		return err
	}
	err = s.b.client.UpdateStackConfig(ctx, stackID, &apitype.StackConfig{
		Environment:     imports[0],
		SecretsProvider: projectStack.SecretsProvider,
		EncryptedKey:    projectStack.EncryptedKey,
		EncryptionSalt:  projectStack.EncryptionSalt,
	})
	return err
}

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
	events chan<- engine.Event,
) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	return backend.PreviewStack(ctx, s, op, events)
}

func (s *cloudStack) Update(
	ctx context.Context,
	op backend.UpdateOperation,
	events chan<- engine.Event,
) (sdkDisplay.ResourceChanges,
	error,
) {
	return backend.UpdateStack(ctx, s, op, events)
}

func (s *cloudStack) Import(ctx context.Context, op backend.UpdateOperation,
	imports []deploy.Import,
) (sdkDisplay.ResourceChanges, error) {
	return backend.ImportStack(ctx, s, op, imports)
}

func (s *cloudStack) Refresh(ctx context.Context, op backend.UpdateOperation) (sdkDisplay.ResourceChanges,
	error,
) {
	return backend.RefreshStack(ctx, s, op)
}

func (s *cloudStack) Destroy(ctx context.Context, op backend.UpdateOperation) (sdkDisplay.ResourceChanges,
	error,
) {
	return backend.DestroyStack(ctx, s, op)
}

func (s *cloudStack) Watch(ctx context.Context, op backend.UpdateOperation, paths []string) error {
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
	// defaultOrg passes a cached value for what the default org should be, through to
	// `cloudBackendReference` where it may be used to qualify the stack name when stringified.
	// If unset, `cloudBackendReference` will refer to the individual org rather than manage
	// an explicit lookup.
	defaultOrg string
}

func (css cloudStackSummary) Name() backend.StackReference {
	contract.Assertf(css.summary.ProjectName != "", "project name must not be empty")
	stackName, err := tokens.ParseStackName(css.summary.StackName)
	contract.AssertNoErrorf(err, "unexpected invalid stack name: %v", css.summary.StackName)

	return cloudBackendReference{
		owner:      css.summary.OrgName,
		defaultOrg: css.defaultOrg,
		project:    tokens.Name(css.summary.ProjectName),
		name:       stackName,
		b:          css.b,
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
