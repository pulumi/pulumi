package service

import service "github.com/pulumi/pulumi/sdk/v3/pkg/secrets/service"

const Type = service.Type

func NewServiceSecretsManager(client *client.Client, id client.StackIdentifier, info *workspace.ProjectStack) (secrets.Manager, error) {
	return service.NewServiceSecretsManager(client, id, info)
}

// NewServiceSecretsManagerFromState returns a Pulumi service-based secrets manager based on the
// existing state.
func NewServiceSecretsManagerFromState(state json.RawMessage) (secrets.Manager, error) {
	return service.NewServiceSecretsManagerFromState(state)
}

