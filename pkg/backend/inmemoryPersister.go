// Copyright 2016-2024, Pulumi Corporation.
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

package backend

import (
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

var _ = SnapshotPersister((*InMemoryPersister)(nil))

type InMemoryPersister struct {
	Snap *deploy.Snapshot
}

func (p *InMemoryPersister) Save(snap *deploy.Snapshot) error {
	result := &deploy.Snapshot{
		Manifest:          snap.Manifest,
		SecretsManager:    snap.SecretsManager,
		Resources:         make([]*resource.State, len(snap.Resources)),
		PendingOperations: make([]resource.Operation, len(snap.PendingOperations)),
	}

	for i, res := range snap.Resources {
		res.Lock.Lock()
		result.Resources[i] = res.Copy()
		res.Lock.Unlock()
	}

	for i, op := range snap.PendingOperations {
		op.Resource.Lock.Lock()
		result.PendingOperations[i] = resource.Operation{
			Type:     op.Type,
			Resource: op.Resource.Copy(),
		}
		op.Resource.Lock.Unlock()
	}

	p.Snap = result
	return nil
}
