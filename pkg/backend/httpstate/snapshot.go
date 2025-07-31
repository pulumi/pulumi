// Copyright 2016-2025, Pulumi Corporation.
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
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// cloudSnapshotPersister persists snapshots to the Pulumi service.
type cloudSnapshotPersister struct {
	context             context.Context         // The context to use for client requests.
	update              client.UpdateIdentifier // The UpdateIdentifier for this update sequence.
	tokenSource         tokenSourceCapability   // A token source for interacting with the service.
	backend             *cloudBackend           // A backend for communicating with the service
	deploymentDiffState *deploymentDiffState
}

func (persister *cloudSnapshotPersister) Save(snapshot *deploy.Snapshot) error {
	ctx := persister.context

	// Diff capability can be nil because of feature flagging.
	if persister.deploymentDiffState == nil {
		untypedDeployment, err := stack.SerializeUntypedDeployment(ctx, snapshot, nil /*opts*/)
		if err != nil {
			return fmt.Errorf("serializing deployment: %w", err)
		}

		persister.backend.downgradeUntypedDeploymentVersionIfNeeded(ctx, untypedDeployment)

		// Continue with how deployments were saved before diff.
		return persister.backend.client.PatchUpdateCheckpoint(
			persister.context, persister.update, untypedDeployment, persister.tokenSource)
	}

	deploymentV3, version, features, err := stack.SerializeDeploymentWithMetadata(ctx, snapshot, false /*showSecrets*/)
	if err != nil {
		return fmt.Errorf("serializing deployment: %w", err)
	}

	version, features = persister.backend.downgradeDeploymentVersionIfNeeded(ctx, version, features)

	differ := persister.deploymentDiffState

	deployment, err := differ.MarshalDeployment(deploymentV3, version, features)
	if err != nil {
		return err
	}

	// If there is no baseline to diff against, or diff is predicted to be inefficient, use saveFull.
	if !differ.ShouldDiff(deployment) {
		if err := persister.saveFullVerbatim(ctx, differ, deployment.raw, version, persister.tokenSource); err != nil {
			return err
		}
	} else { // Otherwise can use saveDiff.
		diff, err := differ.Diff(ctx, deployment)
		if err != nil {
			return err
		}
		if err := persister.saveDiff(ctx, diff, version, persister.tokenSource); err != nil {
			if logging.V(3) {
				logging.V(3).Infof("ignoring error saving checkpoint "+
					"with PatchUpdateCheckpointDelta, falling back to "+
					"PatchUpdateCheckpoint: %v", err)
			}
			if err := persister.saveFullVerbatim(
				ctx, differ, deployment.raw, version, persister.tokenSource); err != nil {
				return err
			}
		}
	}

	return persister.deploymentDiffState.Saved(ctx, deployment)
}

func (persister *cloudSnapshotPersister) saveDiff(ctx context.Context,
	diff deploymentDiff, deploymentVersion int, token client.UpdateTokenSource,
) error {
	return persister.backend.client.PatchUpdateCheckpointDelta(
		persister.context, persister.update,
		diff.sequenceNumber, diff.checkpointHash, diff.deploymentDelta, deploymentVersion, token)
}

func (persister *cloudSnapshotPersister) saveFullVerbatim(ctx context.Context,
	differ *deploymentDiffState, deployment json.RawMessage, deploymentVersion int, token client.UpdateTokenSource,
) error {
	return persister.backend.client.PatchUpdateCheckpointVerbatim(
		persister.context, persister.update, differ.SequenceNumber(),
		deployment, deploymentVersion, token)
}

var _ backend.SnapshotPersister = (*cloudSnapshotPersister)(nil)

func (b *cloudBackend) newSnapshotPersister(ctx context.Context, update client.UpdateIdentifier,
	tokenSource tokenSourceCapability,
) *cloudSnapshotPersister {
	p := &cloudSnapshotPersister{
		context:     ctx,
		update:      update,
		tokenSource: tokenSource,
		backend:     b,
	}

	caps := b.Capabilities(ctx)
	deltaCaps := caps.DeltaCheckpointUpdates
	if deltaCaps != nil {
		p.deploymentDiffState = newDeploymentDiffState(deltaCaps.CheckpointCutoffSizeBytes)
	}
	return p
}
