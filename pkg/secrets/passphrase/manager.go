package passphrase

import passphrase "github.com/pulumi/pulumi/sdk/v3/pkg/secrets/passphrase"

const Type = passphrase.Type

var ErrIncorrectPassphrase = passphrase.ErrIncorrectPassphrase

func EditProjectStack(info *workspace.ProjectStack, state json.RawMessage) error {
	return passphrase.EditProjectStack(info, state)
}

func NewPassphraseSecretsManager(phrase string) (string, secrets.Manager, error) {
	return passphrase.NewPassphraseSecretsManager(phrase)
}

func GetPassphraseSecretsManager(phrase string, state string) (secrets.Manager, error) {
	return passphrase.GetPassphraseSecretsManager(phrase, state)
}

// NewPassphraseSecretsManager returns a new passphrase-based secrets manager, from the
// given state. Will use the passphrase found in PULUMI_CONFIG_PASSPHRASE, the file specified by
// PULUMI_CONFIG_PASSPHRASE_FILE, or otherwise will prompt for the passphrase if interactive.
func NewPromptingPassphraseSecretsManagerFromState(state json.RawMessage) (secrets.Manager, error) {
	return passphrase.NewPromptingPassphraseSecretsManagerFromState(state)
}

// NewStackPromptingPassphraseSecretsManager returns a new passphrase-based secrets manager, from the
// given state. Will use the passphrase found in PULUMI_CONFIG_PASSPHRASE, the file specified by
// PULUMI_CONFIG_PASSPHRASE_FILE, or otherwise will prompt for the passphrase if interactive.
// It also takes a stack name to include in the prompt.
func NewStackPromptingPassphraseSecretsManagerFromState(state json.RawMessage, stackName string) (secrets.Manager, error) {
	return passphrase.NewStackPromptingPassphraseSecretsManagerFromState(state, stackName)
}

func NewPromptingPassphraseSecretsManager(info *workspace.ProjectStack, rotateSecretsProvider bool) (secrets.Manager, error) {
	return passphrase.NewPromptingPassphraseSecretsManager(info, rotateSecretsProvider)
}

