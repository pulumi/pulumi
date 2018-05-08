// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"context"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud/client"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
)

// cloudSnapshotPersister persists snapshots to the Pulumi service.
type cloudSnapshotPersister struct {
	context     context.Context         // The context to use for client requests.
	update      client.UpdateIdentifier // The UpdateIdentifier for this update sequence.
	tokenSource *tokenSource            // A token source for interacting with the service.
	backend     *cloudBackend           // A backend for communicating with the service
}

func (persister *cloudSnapshotPersister) Invalidate() error {
	token, err := persister.tokenSource.GetToken()
	if err != nil {
		return err
	}

	return persister.backend.client.InvalidateUpdateCheckpoint(persister.context, persister.update, token)
}

func (persister *cloudSnapshotPersister) Save(snapshot *deploy.Snapshot) error {
	token, err := persister.tokenSource.GetToken()
	if err != nil {
		return err
	}
	deployment := stack.SerializeDeployment(snapshot)
	return persister.backend.client.PatchUpdateCheckpoint(persister.context, persister.update, deployment, token)
}

var _ backend.SnapshotPersister = (*cloudSnapshotPersister)(nil)

func (cb *cloudBackend) newSnapshotPersister(ctx context.Context, update client.UpdateIdentifier,
	tokenSource *tokenSource) *cloudSnapshotPersister {
	return &cloudSnapshotPersister{
		context:     ctx,
		update:      update,
		tokenSource: tokenSource,
		backend:     cb,
	}
}
