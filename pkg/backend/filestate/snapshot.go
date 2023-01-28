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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// localSnapshotManager is a simple SnapshotManager implementation that persists snapshots
// to disk on the local machine.
type localSnapshotPersister struct {
	name    tokens.QName
	backend *localBackend
	sm      secrets.Manager
}

func (sp *localSnapshotPersister) SecretsManager() secrets.Manager {
	return sp.sm
}

func (sp *localSnapshotPersister) Save(snapshot *deploy.Snapshot) error {
	_, err := sp.backend.saveStack(sp.name, snapshot, sp.sm)
	return err

}

func (b *localBackend) newSnapshotPersister(stackName tokens.QName, sm secrets.Manager) *localSnapshotPersister {
	return &localSnapshotPersister{name: stackName, backend: b, sm: sm}
}
