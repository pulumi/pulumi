// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"os"

	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// localSnapshotManager is a simple SnapshotManager implementation that persists snapshots
// to disk on the local machine.
type localSnapshotPersister struct {
	name    tokens.QName
	backend *localBackend
}

func (sm *localSnapshotPersister) Invalidate() error {
	return nil
}

func (sm *localSnapshotPersister) Save(snapshot *deploy.Snapshot) error {
	stack := snapshot.Stack
	contract.Assert(sm.name == stack)

	config, _, _, err := sm.backend.getStack(stack)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	_, err = sm.backend.saveStack(stack, config, snapshot)
	return err

}

func (b *localBackend) newSnapshotPersister(stackName tokens.QName) *localSnapshotPersister {
	return &localSnapshotPersister{name: stackName, backend: b}
}
