package b64

import b64 "github.com/pulumi/pulumi/sdk/v3/pkg/secrets/b64"

// Base64SecretsProvider is a SecretsProvider that only supports base64 secrets, it is intended to be used for tests
// where actual encryption is not needed.
var Base64SecretsProvider = b64.Base64SecretsProvider

