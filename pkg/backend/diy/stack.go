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

package diy

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// diyStack is a diy stack descriptor.
type diyStack struct {
	// the stack's reference (qualified name).
	ref *diyBackendReference
	// a snapshot representing the latest deployment state, allocated on first use. It's valid for the
	// snapshot itself to be nil.
	snapshot atomic.Pointer[*deploy.Snapshot]
	// a pointer to the backend this stack belongs to.
	b *diyBackend
}

func newStack(ref *diyBackendReference, b *diyBackend) backend.Stack {
	contract.Requiref(ref != nil, "ref", "ref was nil")

	return &diyStack{
		ref: ref,
		b:   b,
	}
}

func (s *diyStack) Ref() backend.StackReference                 { return s.ref }
func (s *diyStack) ConfigLocation() backend.StackConfigLocation { return backend.StackConfigLocation{} }

func (s *diyStack) LoadRemoteConfig(ctx context.Context, project *workspace.Project) (*workspace.ProjectStack, error) {
	return nil, errors.New("remote config not implemented for the DIY backend")
}

func (s *diyStack) SaveRemoteConfig(ctx context.Context, projectStack *workspace.ProjectStack) error {
	// TODO: https://github.com/pulumi/pulumi/issues/19557
	return errors.New("remote config not implemented for the DIY backend")
}

func (s *diyStack) Snapshot(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
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
func (s *diyStack) Backend() backend.Backend              { return s.b }
func (s *diyStack) Tags() map[apitype.StackTagName]string { return nil }

func (s *diyStack) DefaultSecretManager(info *workspace.ProjectStack) (secrets.Manager, error) {
	return passphrase.NewPromptingPassphraseSecretsManager(info, false /* rotatePassphraseSecretsProvider */)
}

type diyStackSummary struct {
	name backend.StackReference
	chk  *apitype.CheckpointV3
}

func newDIYStackSummary(name backend.StackReference, chk *apitype.CheckpointV3) diyStackSummary {
	return diyStackSummary{name: name, chk: chk}
}

func (lss diyStackSummary) Name() backend.StackReference {
	return lss.name
}

func (lss diyStackSummary) LastUpdate() *time.Time {
	if lss.chk != nil && lss.chk.Latest != nil {
		if t := lss.chk.Latest.Manifest.Time; !t.IsZero() {
			return &t
		}
	}
	return nil
}

func (lss diyStackSummary) ResourceCount() *int {
	if lss.chk != nil && lss.chk.Latest != nil {
		count := len(lss.chk.Latest.Resources)
		return &count
	}
	return nil
}
