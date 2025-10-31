package stack

import stack "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/stack"

// A SecretsManagerLoader provides methods for loading secrets managers and
// their encrypters and decrypters for a given stack and project stack. A loader
// encapsulates the logic for determining which secrets manager to use based on
// a given configuration, such as whether or not to fallback to the stack state
// if there is no secrets manager configured in the project stack.
type SecretsManagerLoader = stack.SecretsManagerLoader

// The state of a stack's secret manager configuration following an operation.
type SecretsManagerState = stack.SecretsManagerState

const SecretsManagerUnchanged = stack.SecretsManagerUnchanged

const SecretsManagerShouldSave = stack.SecretsManagerShouldSave

const SecretsManagerMustSave = stack.SecretsManagerMustSave

// Creates a secrets manager for an existing stack, using the stack to pick defaults if necessary and writing any
// changes back to the stack's configuration where applicable.
func CreateSecretsManagerForExistingStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stack_ backend.Stack, secretsProvider string, rotateSecretsProvider, creatingStack bool) error {
	return stack.CreateSecretsManagerForExistingStack(ctx, sink, ws, stack_, secretsProvider, rotateSecretsProvider, creatingStack)
}

// Creates a new stack secrets manager loader from the environment.
func NewStackSecretsManagerLoaderFromEnv() SecretsManagerLoader {
	return stack.NewStackSecretsManagerLoaderFromEnv()
}

func ValidateSecretsProvider(typ string) error {
	return stack.ValidateSecretsProvider(typ)
}

// we only want to log a secrets decryption for a Pulumi Cloud backend project
// we will allow any secrets provider to be used (Pulumi Cloud or passphrase/cloud/etc)
// we will log the message and not worry about the response. The types
// of messages we will log here will range from single secret decryption events
// to requesting a list of secrets in an individual event e.g. stack export
// the logging event will only happen during the `--show-secrets` path within the cli
func Log3rdPartySecretsProviderDecryptionEvent(ctx context.Context, backend backend.Stack, secretName, commandName string) {
	stack.Log3rdPartySecretsProviderDecryptionEvent(ctx, backend, secretName, commandName)
}

