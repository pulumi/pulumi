package b64

import b64 "github.com/pulumi/pulumi/sdk/v3/pkg/secrets/b64"

const Type = b64.Type

// NewBase64SecretsManager returns a secrets manager that just base64 encodes instead of encrypting. Useful for testing.
func NewBase64SecretsManager() secrets.Manager {
	return b64.NewBase64SecretsManager()
}

