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
	"sync/atomic"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// localStack is a local stack descriptor.
type localStack struct {
	// the stack's reference (qualified name).
	ref *localBackendReference
	// a snapshot representing the latest deployment state, allocated on first use. It's valid for the
	// snapshot itself to be nil.
	snapshot atomic.Pointer[*deploy.Snapshot]
	// a pointer to the backend this stack belongs to.
	b *localBackend
}

func newStack(ref *localBackendReference, b *localBackend) backend.Stack {
	contract.Requiref(ref != nil, "ref", "ref was nil")

	return &localStack{
		ref: ref,
		b:   b,
	}
}

func (s *localStack) Ref() backend.StackReference { return s.ref }
func (s *localStack) Snapshot(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
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
func (s *localStack) Backend() backend.Backend              { return s.b }
func (s *localStack) Tags() map[apitype.StackTagName]string { return nil }

func (s *localStack) Remove(ctx context.Context, force bool) (bool, error) {
	return backend.RemoveStack(ctx, s, force)
}

func (s *localStack) Rename(ctx context.Context, newName tokens.QName) (backend.StackReference, error) {
	return backend.RenameStack(ctx, s, newName)
}

func (s *localStack) Preview(
	ctx context.Context,
	op backend.UpdateOperation,
) (*deploy.Plan, display.ResourceChanges, result.Result) {
	return backend.PreviewStack(ctx, s, op)
}

func (s *localStack) Update(ctx context.Context, op backend.UpdateOperation) (display.ResourceChanges, result.Result) {
	return backend.UpdateStack(ctx, s, op)
}

func (s *localStack) Import(ctx context.Context, op backend.UpdateOperation,
	imports []deploy.Import,
) (display.ResourceChanges, result.Result) {
	return backend.ImportStack(ctx, s, op, imports)
}

func (s *localStack) Refresh(ctx context.Context, op backend.UpdateOperation) (display.ResourceChanges, result.Result) {
	return backend.RefreshStack(ctx, s, op)
}

func (s *localStack) Destroy(ctx context.Context, op backend.UpdateOperation) (display.ResourceChanges, result.Result) {
	return backend.DestroyStack(ctx, s, op)
}

func (s *localStack) Watch(ctx context.Context, op backend.UpdateOperation, paths []string) result.Result {
	return backend.WatchStack(ctx, s, op, paths)
}

func (s *localStack) GetLogs(ctx context.Context, secretsProvider secrets.Provider, cfg backend.StackConfiguration,
	query operations.LogQuery,
) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(ctx, secretsProvider, s, cfg, query)
}

func (s *localStack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(ctx, s)
}

func (s *localStack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(ctx, s, deployment)
}

func (s *localStack) DefaultSecretManager(info *workspace.ProjectStack) (secrets.Manager, error) {
	return passphrase.NewPromptingPassphraseSecretsManager(info, false /* rotatePassphraseSecretsProvider */)
}

type localStackSummary struct {
	name backend.StackReference
	chk  *apitype.CheckpointV3
}

func newLocalStackSummary(name backend.StackReference, chk *apitype.CheckpointV3) localStackSummary {
	return localStackSummary{name: name, chk: chk}
}

func (lss localStackSummary) Name() backend.StackReference {
	return lss.name
}

func (lss localStackSummary) LastUpdate() *time.Time {
	if lss.chk != nil && lss.chk.Latest != nil {
		if t := lss.chk.Latest.Manifest.Time; !t.IsZero() {
			return &t
		}
	}
	return nil
}

func (lss localStackSummary) ResourceCount() *int {
	if lss.chk != nil && lss.chk.Latest != nil {
		count := len(lss.chk.Latest.Resources)
		return &count
	}
	return nil
}
