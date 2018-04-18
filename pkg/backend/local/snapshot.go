// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"os"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// localSnapshotManager is a simple SnapshotManager implementation that persists snapshots
// to disk on the local machine.
type localSnapshotManager struct {
	name    tokens.QName
	backend *localBackend
}

// BeginMutation does nothing and returns a SnapshotMutation that, when `End`ed,
// saves a new snapshot to disk.
func (sm *localSnapshotManager) BeginMutation() (engine.SnapshotMutation, error) {
	return &localSnapshotMutation{manager: sm}, nil
}

func (sm *localSnapshotManager) Close() error { return nil }

var _ engine.SnapshotManager = (*localSnapshotManager)(nil)

type localSnapshotMutation struct {
	manager *localSnapshotManager
}

func (lsm *localSnapshotMutation) End(snapshot *deploy.Snapshot) error {
	stack := snapshot.Stack
	contract.Assert(lsm.manager.name == stack)

	config, _, _, err := lsm.manager.backend.getStack(stack)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	_, err = lsm.manager.backend.saveStack(stack, config, snapshot)
	return err
}

var _ engine.SnapshotMutation = (*localSnapshotMutation)(nil)

func (b *localBackend) newSnapshotManager(stackName tokens.QName) *localSnapshotManager {
	return &localSnapshotManager{name: stackName, backend: b}
}
