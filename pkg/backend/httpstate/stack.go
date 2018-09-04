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

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack is a cloud stack.  This simply adds some cloud-specific properties atop the standard backend stack interface.
type Stack interface {
	backend.Stack
	CloudURL() string            // the URL to the cloud containing this stack.
	OrgName() string             // the organization that owns this stack.
	ConsoleURL() (string, error) // the URL to view the stack's information on Pulumi.com
}

// cloudStack is a cloud stack descriptor.
type cloudStack struct {
	name     backend.StackReference // the stack's name.
	cloudURL string                 // the URL to the cloud containing this stack.
	orgName  string                 // the organization that owns this stack.
	config   config.Map             // the stack's config bag.
	snapshot **deploy.Snapshot      // a snapshot representing the latest deployment state (allocated on first use)
	b        *cloudBackend          // a pointer to the backend this stack belongs to.
}

type cloudBackendReference struct {
	name  tokens.QName
	owner string
	b     *cloudBackend
}

func (c cloudBackendReference) String() string {
	curUser, err := c.b.client.GetPulumiAccountName(context.Background())
	if err != nil {
		curUser = ""
	}

	if c.owner == curUser {
		return string(c.name)
	}

	return fmt.Sprintf("%s/%s", c.owner, c.name)
}

func (c cloudBackendReference) StackName() tokens.QName {
	return c.name
}

func newStack(apistack apitype.Stack, b *cloudBackend) Stack {
	// Now assemble all the pieces into a stack structure.
	return &cloudStack{
		name: cloudBackendReference{
			owner: apistack.OrgName,
			name:  apistack.StackName,
			b:     b,
		},
		cloudURL: b.CloudURL(),
		orgName:  apistack.OrgName,
		config:   nil, // TODO[pulumi/pulumi-service#249]: add the config variables.
		snapshot: nil, // We explicitly allocate the snapshot on first use, since it is expensive to compute.
		b:        b,
	}
}

func (s *cloudStack) Name() backend.StackReference { return s.name }
func (s *cloudStack) Config() config.Map           { return s.config }
func (s *cloudStack) Backend() backend.Backend     { return s.b }
func (s *cloudStack) CloudURL() string             { return s.cloudURL }
func (s *cloudStack) OrgName() string              { return s.orgName }

func (s *cloudStack) Snapshot(ctx context.Context) (*deploy.Snapshot, error) {
	if s.snapshot != nil {
		return *s.snapshot, nil
	}

	snap, err := s.b.getSnapshot(ctx, s.name)
	if err != nil {
		return nil, err
	}

	s.snapshot = &snap
	return *s.snapshot, nil
}

func (s *cloudStack) Remove(ctx context.Context, force bool) (bool, error) {
	return backend.RemoveStack(ctx, s, force)
}

func (s *cloudStack) Preview(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.PreviewStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *cloudStack) Update(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.UpdateStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *cloudStack) Refresh(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.RefreshStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *cloudStack) Destroy(ctx context.Context, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return backend.DestroyStack(ctx, s, proj, root, m, opts, scopes)
}

func (s *cloudStack) GetLogs(ctx context.Context, query operations.LogQuery) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(ctx, s, query)
}

func (s *cloudStack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(ctx, s)
}

func (s *cloudStack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(ctx, s, deployment)
}

func (s *cloudStack) ConsoleURL() (string, error) {
	path, err := s.b.StackConsoleURL(s.Name())
	if err != nil {
		return "", nil
	}
	url := s.b.CloudConsoleURL(path)
	if url == "" {
		return "", errors.New("could not determine clould console URL")
	}
	return url, nil
}
