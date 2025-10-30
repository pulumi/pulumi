package cloud

import cloud "github.com/pulumi/pulumi/sdk/v3/pkg/secrets/cloud"

// Manager is the secrets.Manager implementation for cloud key management services
type Manager = cloud.Manager

// Type is the type of secrets managed by this secrets provider
const Type = cloud.Type

func EditProjectStack(info *workspace.ProjectStack, state json.RawMessage) error {
	return cloud.EditProjectStack(info, state)
}

// NewCloudSecretsManagerFromState deserialize configuration from state and returns a secrets
// manager that uses the target cloud key management service to encrypt/decrypt a data key used for
// envelope encryption of secrets values.
func NewCloudSecretsManagerFromState(state json.RawMessage) (secrets.Manager, error) {
	return cloud.NewCloudSecretsManagerFromState(state)
}

func NewCloudSecretsManager(info *workspace.ProjectStack, secretsProvider string, rotateSecretsProvider bool) (secrets.Manager, error) {
	return cloud.NewCloudSecretsManager(info, secretsProvider, rotateSecretsProvider)
}

