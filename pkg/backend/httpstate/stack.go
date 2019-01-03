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
	"time"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Stack is a cloud stack.  This simply adds some cloud-specific properties atop the standard backend stack interface.
type Stack interface {
	backend.Stack
	CloudURL() string                               // the URL to the cloud containing this stack.
	OrgName() string                                // the organization that owns this stack.
	ConsoleURL() (string, error)                    // the URL to view the stack's information on Pulumi.com
	Tags() map[apitype.StackTagName]string          // the stack's tags.
	MergeTags(tags map[apitype.StackTagName]string) // merges tags with the stack's existing tags.
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

func (c cloudBackendReference) Name() tokens.QName {
	return c.name
}

// cloudStack is a cloud stack descriptor.
type cloudStack struct {
	// ref is the stack's unique name.
	ref backend.StackReference
	// cloudURL is the URl to the cloud containing this stack.
	cloudURL string
	// orgName is the organization that owns this stack.
	orgName string
	// config is this stack's config bag.
	config config.Map
	// snapshot contains the latest deployment state, allocated on first use.
	snapshot **deploy.Snapshot
	// b is a pointer to the backend that this stack belongs to.
	b *cloudBackend
	// tags contains metadata tags describing additional, extensible properties about this stack.
	tags map[apitype.StackTagName]string
}

func newStack(apistack apitype.Stack, b *cloudBackend) Stack {
	// Now assemble all the pieces into a stack structure.
	return &cloudStack{
		ref: cloudBackendReference{
			owner: apistack.OrgName,
			name:  apistack.StackName,
			b:     b,
		},
		cloudURL: b.CloudURL(),
		orgName:  apistack.OrgName,
		config:   nil, // TODO[pulumi/pulumi-service#249]: add the config variables.
		snapshot: nil, // We explicitly allocate the snapshot on first use, since it is expensive to compute.
		tags:     apistack.Tags,
		b:        b,
	}
}

func (s *cloudStack) Ref() backend.StackReference           { return s.ref }
func (s *cloudStack) Config() config.Map                    { return s.config }
func (s *cloudStack) Backend() backend.Backend              { return s.b }
func (s *cloudStack) CloudURL() string                      { return s.cloudURL }
func (s *cloudStack) OrgName() string                       { return s.orgName }
func (s *cloudStack) Tags() map[apitype.StackTagName]string { return s.tags }

func (s *cloudStack) MergeTags(tags map[apitype.StackTagName]string) {
	if len(tags) == 0 {
		return
	}

	if s.tags == nil {
		s.tags = make(map[apitype.StackTagName]string)
	}

	// Add each new tag to the existing tags, overwriting existing tags with the
	// latest values.
	for k, v := range tags {
		s.tags[k] = v
	}
}

func (s *cloudStack) Snapshot(ctx context.Context) (*deploy.Snapshot, error) {
	if s.snapshot != nil {
		return *s.snapshot, nil
	}

	snap, err := s.b.getSnapshot(ctx, s.ref)
	if err != nil {
		return nil, err
	}

	s.snapshot = &snap
	return *s.snapshot, nil
}

func (s *cloudStack) Remove(ctx context.Context, force bool) (bool, error) {
	return backend.RemoveStack(ctx, s, force)
}

func (s *cloudStack) Preview(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.PreviewStack(ctx, s, op)
}

func (s *cloudStack) Update(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.UpdateStack(ctx, s, op)
}

func (s *cloudStack) Refresh(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.RefreshStack(ctx, s, op)
}

func (s *cloudStack) Destroy(ctx context.Context, op backend.UpdateOperation) (engine.ResourceChanges, error) {
	return backend.DestroyStack(ctx, s, op)
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
	return s.b.StackConsoleURL(s.ref)
}

// cloudStackSummary implements the backend.StackSummary interface, by wrapping
// an apitype.StackSummary struct.
type cloudStackSummary struct {
	summary apitype.StackSummary
	b       *cloudBackend
}

func (css cloudStackSummary) Name() backend.StackReference {
	return cloudBackendReference{
		owner: css.summary.OrgName,
		name:  tokens.QName(css.summary.StackName),
		b:     css.b,
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
