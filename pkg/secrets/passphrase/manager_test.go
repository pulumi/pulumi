package passphrase

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	state = `
    {"salt": "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}
`
	brokenState = `
    {"salt": "fozI5u6B030=:v1:F+6ZduL:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}
`
)

func resetPassphraseTestEnvVars() func() {
	clearCachedSecretsManagers()

	oldPassphrase := os.Getenv("PULUMI_CONFIG_PASSPHRASE")
	oldPassphraseFile := os.Getenv("PULUMI_CONFIG_PASSPHRASE_FILE")
	return func() {
		os.Setenv("PULUMI_CONFIG_PASSPHRASE", oldPassphrase)
		os.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", oldPassphraseFile)
	}
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseManagerIncorrectPassphraseReturnsErrorCrypter(t *testing.T) {
	resetEnv := resetPassphraseTestEnvVars()
	defer resetEnv()

	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "password123")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	manager, err := NewPromptingPassphaseSecretsManagerFromState([]byte(state))
	assert.NoError(t, err) // even if we pass the wrong provider, we should get a lockedPassphraseProvider

	assert.Equal(t, manager, &localSecretsManager{
		state:   localSecretsManagerState{Salt: "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="},
		crypter: &errorCrypter{},
	})
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseManagerIncorrectStateReturnsError(t *testing.T) {
	resetEnv := resetPassphraseTestEnvVars()
	defer resetEnv()

	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "password")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	_, err := NewPromptingPassphaseSecretsManagerFromState([]byte(brokenState))
	assert.Error(t, err)
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseManagerCorrectPassphraseReturnsSecretsManager(t *testing.T) {
	resetEnv := resetPassphraseTestEnvVars()
	defer resetEnv()

	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "password")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	sm, err := NewPromptingPassphaseSecretsManagerFromState([]byte(state))
	assert.NoError(t, err)
	assert.NotNil(t, sm)
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseManagerNoEnvironmentVariablesReturnsError(t *testing.T) {
	resetEnv := resetPassphraseTestEnvVars()
	defer resetEnv()

	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	_, err := NewPromptingPassphaseSecretsManagerFromState([]byte(state))
	assert.NotNil(t, err, strings.Contains(err.Error(), "unable to find either `PULUMI_CONFIG_PASSPHRASE` nor "+
		"`PULUMI_CONFIG_PASSPHRASE_FILE`"))
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseManagerEmptyPassphraseIsValid(t *testing.T) {
	resetEnv := resetPassphraseTestEnvVars()
	defer resetEnv()

	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	sm, err := NewPromptingPassphaseSecretsManagerFromState([]byte(state))
	assert.NoError(t, err)
	assert.NotNil(t, sm)
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseManagerCorrectPassfileReturnsSecretsManager(t *testing.T) {
	resetEnv := resetPassphraseTestEnvVars()
	defer resetEnv()

	tmpFile, err := os.CreateTemp("", "pulumi-secret-test")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString("password")
	assert.NoError(t, err)

	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
	os.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", tmpFile.Name())

	sm, err := NewPromptingPassphaseSecretsManagerFromState([]byte(state))
	assert.NoError(t, err)
	assert.NotNil(t, sm)
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseManagerEmptyPassfileReturnsError(t *testing.T) {
	resetEnv := resetPassphraseTestEnvVars()
	defer resetEnv()

	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
	os.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")

	_, err := NewPromptingPassphaseSecretsManagerFromState([]byte(state))
	assert.NotNil(t, err, strings.Contains(err.Error(), "unable to find either `PULUMI_CONFIG_PASSPHRASE` nor "+
		"`PULUMI_CONFIG_PASSPHRASE_FILE`"))
}
