// Copyright 2016-2022, Pulumi Corporation.
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

package stack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype/migrate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func UnmarshalVersionedCheckpointToLatestCheckpoint(m encoding.Marshaler, bytes []byte) (*apitype.CheckpointV3, error) {
	var versionedCheckpoint apitype.VersionedCheckpoint
	// Here we are careful to unmarshal `bytes` with the provided unmarshaller `m`.
	if err := m.Unmarshal(bytes, &versionedCheckpoint); err != nil {
		return nil, fmt.Errorf("place 1: %w", err)
	}

	switch versionedCheckpoint.Version {
	case 0:
		// The happens when we are loading a checkpoint file from before we started to version things. Go's
		// json package did not support strict marshalling before 1.10, and we use 1.9 in our toolchain today.
		// After we upgrade, we could consider rewriting this code to use DisallowUnknownFields() on the decoder
		// to have the old checkpoint not even deserialize as an apitype.VersionedCheckpoint.
		var v1checkpoint apitype.CheckpointV1
		if err := m.Unmarshal(bytes, &v1checkpoint); err != nil {
			return nil, err
		}

		v2checkpoint := migrate.UpToCheckpointV2(v1checkpoint)
		v3checkpoint := migrate.UpToCheckpointV3(v2checkpoint)
		return &v3checkpoint, nil
	case 1:
		var v1checkpoint apitype.CheckpointV1
		if err := json.Unmarshal(versionedCheckpoint.Checkpoint, &v1checkpoint); err != nil {
			return nil, err
		}

		v2checkpoint := migrate.UpToCheckpointV2(v1checkpoint)
		v3checkpoint := migrate.UpToCheckpointV3(v2checkpoint)
		return &v3checkpoint, nil
	case 2:
		var v2checkpoint apitype.CheckpointV2
		if err := json.Unmarshal(versionedCheckpoint.Checkpoint, &v2checkpoint); err != nil {
			return nil, err
		}

		v3checkpoint := migrate.UpToCheckpointV3(v2checkpoint)
		return &v3checkpoint, nil
	case 3:
		var v3checkpoint apitype.CheckpointV3
		if err := json.Unmarshal(versionedCheckpoint.Checkpoint, &v3checkpoint); err != nil {
			return nil, err
		}

		return &v3checkpoint, nil
	default:
		return nil, fmt.Errorf("unsupported checkpoint version %d", versionedCheckpoint.Version)
	}
}

func MarshalUntypedDeploymentToVersionedCheckpoint(
	stack tokens.QName, deployment *apitype.UntypedDeployment,
) (*apitype.VersionedCheckpoint, error) {
	chk := struct {
		Stack  tokens.QName
		Latest json.RawMessage
	}{
		Stack:  stack,
		Latest: deployment.Deployment,
	}

	bytes, err := encoding.JSON.Marshal(chk)
	if err != nil {
		return nil, fmt.Errorf("marshalling checkpoint: %w", err)
	}

	return &apitype.VersionedCheckpoint{
		Version:    deployment.Version,
		Checkpoint: bytes,
	}, nil
}

// SerializeCheckpoint turns a snapshot into a data structure suitable for serialization.
func SerializeCheckpoint(stack tokens.QName, snap *deploy.Snapshot,
	showSecrets bool,
) (*apitype.VersionedCheckpoint, error) {
	// If snap is nil, that's okay, we will just create an empty deployment; otherwise, serialize the whole snapshot.
	var latest *apitype.DeploymentV3
	if snap != nil {
		ctx := context.TODO()
		dep, err := SerializeDeployment(ctx, snap, showSecrets)
		if err != nil {
			return nil, fmt.Errorf("serializing deployment: %w", err)
		}
		latest = dep
	}

	b, err := encoding.JSON.Marshal(apitype.CheckpointV3{
		Stack:  stack,
		Latest: latest,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling checkpoint: %w", err)
	}

	return &apitype.VersionedCheckpoint{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Checkpoint: json.RawMessage(b),
	}, nil
}

// DeserializeCheckpoint takes a serialized deployment record and returns its associated snapshot. Returns nil
// if there have been no deployments performed on this checkpoint.
func DeserializeCheckpoint(
	ctx context.Context,
	secretsProvider secrets.Provider,
	chkpoint *apitype.CheckpointV3,
) (*deploy.Snapshot, error) {
	contract.Requiref(chkpoint != nil, "chkpoint", "must not be nil")
	if chkpoint.Latest != nil {
		return DeserializeDeploymentV3(ctx, *chkpoint.Latest, secretsProvider)
	}

	return nil, nil
}

// GetRootStackResource returns the root stack resource from a given snapshot, or nil if not found.
func GetRootStackResource(snap *deploy.Snapshot) (*resource.State, error) {
	if snap != nil {
		for _, res := range snap.Resources {
			if res.Type == resource.RootStackType && res.Parent == "" {
				return res, nil
			}
		}
	}
	return nil, nil
}

// CreateRootStackResource creates a new root stack resource with the given name
func CreateRootStackResource(stackName tokens.QName, projectName tokens.PackageName) *resource.State {
	typ, name := resource.RootStackType, fmt.Sprintf("%s-%s", projectName, stackName)
	urn := resource.NewURN(stackName, projectName, "", typ, name)
	state := resource.NewState(typ, urn, false, false, "", resource.PropertyMap{}, nil, "", false, false, nil, nil, "",
		nil, false, nil, nil, nil, "", false, "", nil, nil, "", nil, nil, nil)
	return state
}
