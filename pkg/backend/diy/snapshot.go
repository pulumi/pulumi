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

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

// diySnapshotPersister is a simple SnapshotManager implementation that persists snapshots
// to blob storage.
type diySnapshotPersister struct {
	// TODO[pulumi/pulumi#12593]:
	// Remove this once SnapshotPersister is updated to take a context.
	ctx context.Context

	ref     *diyBackendReference
	backend *diyBackend
}

func (sp *diySnapshotPersister) Save(snapshot *deploy.Snapshot) error {
	_, err := sp.backend.saveStack(sp.ctx, sp.ref, snapshot)
	return err
}

func (b *diyBackend) newSnapshotPersister(
	ctx context.Context,
	ref *diyBackendReference,
) *diySnapshotPersister {
	return &diySnapshotPersister{ctx: ctx, ref: ref, backend: b}
}
