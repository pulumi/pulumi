// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"github.com/pulumi/pulumi/pkg/backend/cloud/client"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
)

// cloudSnapshotManager persists snapshots to the Pulumi service.
type cloudSnapshotManager struct {
	update      client.UpdateIdentifier // The UpdateIdentifier for this update sequence.
	tokenSource *tokenSource            // A token source for interacting with the service.
	backend     *cloudBackend           // A backend for communicating with the service
}

// BeginMutation marks the snapshot with an intent to mutate it by invalidating the existing
// saved checkpoint.
func (csm *cloudSnapshotManager) BeginMutation() (engine.SnapshotMutation, error) {
	// invalidate the current checkpoint
	token, err := csm.tokenSource.GetToken()
	if err != nil {
		return nil, err
	}
	if err = csm.backend.client.InvalidateUpdateCheckpoint(csm.update, token); err != nil {
		return nil, err
	}
	return &cloudSnapshotMutation{manager: csm}, nil
}

func (csm *cloudSnapshotManager) Close() error { return nil }

var _ engine.SnapshotManager = (*cloudSnapshotManager)(nil)

// cloudSnapshotMutation represents a single mutating operation on the checkpoint. `End` completes
// the mutation sequence by "patching" the checkpoint with the new snapshot and removing the "dirty"
// bit set by `BeginMutation`.
type cloudSnapshotMutation struct {
	manager *cloudSnapshotManager
}

func (csm *cloudSnapshotMutation) End(snapshot *deploy.Snapshot) error {
	// Upload the new checkpoint.
	token, err := csm.manager.tokenSource.GetToken()
	if err != nil {
		return err
	}
	deployment := stack.SerializeDeployment(snapshot)
	return csm.manager.backend.client.PatchUpdateCheckpoint(csm.manager.update, deployment, token)
}

var _ engine.SnapshotMutation = (*cloudSnapshotMutation)(nil)

func (cb *cloudBackend) newSnapshotManager(update client.UpdateIdentifier,
	tokenSource *tokenSource) *cloudSnapshotManager {
	return &cloudSnapshotManager{
		update:      update,
		tokenSource: tokenSource,
		backend:     cb,
	}
}
