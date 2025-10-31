package stack

import stack "github.com/pulumi/pulumi/sdk/v3/pkg/resource/stack"

// UnmarshalVersionedCheckpointToLatestCheckpoint unmarshals a versioned checkpoint to the latest checkpoint format.
// It returns the checkpoint, its version, and any features, or an error if the checkpoint is not valid, cannot be
// migrated to the latest version, or if the features are not supported.
func UnmarshalVersionedCheckpointToLatestCheckpoint(m encoding.Marshaler, bytes []byte) (*apitype.CheckpointV3, int, []string, error) {
	return stack.UnmarshalVersionedCheckpointToLatestCheckpoint(m, bytes)
}

func MarshalUntypedDeploymentToVersionedCheckpoint(stack_ tokens.QName, deployment *apitype.UntypedDeployment) (*apitype.VersionedCheckpoint, error) {
	return stack.MarshalUntypedDeploymentToVersionedCheckpoint(stack_, deployment)
}

// SerializeCheckpoint turns a snapshot into a data structure suitable for serialization.
func SerializeCheckpoint(stack_ tokens.QName, snap *deploy.Snapshot, showSecrets bool) (*apitype.VersionedCheckpoint, error) {
	return stack.SerializeCheckpoint(stack_, snap, showSecrets)
}

// DeserializeCheckpoint takes a serialized deployment record and returns its associated snapshot. Returns nil
// if there have been no deployments performed on this checkpoint.
func DeserializeCheckpoint(ctx context.Context, secretsProvider secrets.Provider, chkpoint *apitype.CheckpointV3) (*deploy.Snapshot, error) {
	return stack.DeserializeCheckpoint(ctx, secretsProvider, chkpoint)
}

// GetRootStackResource returns the root stack resource from a given snapshot, or nil if not found.
func GetRootStackResource(snap *deploy.Snapshot) (*resource.State, error) {
	return stack.GetRootStackResource(snap)
}

// CreateRootStackResource creates a new root stack resource with the given name
func CreateRootStackResource(stackName tokens.QName, projectName tokens.PackageName) *resource.State {
	return stack.CreateRootStackResource(stackName, projectName)
}

