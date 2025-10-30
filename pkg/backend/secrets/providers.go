package secrets

import secrets "github.com/pulumi/pulumi/sdk/v3/pkg/backend/secrets"

// NamedStackProvider is the same as the default secrets provider,
// but is aware of the stack name for which it is used.  Currently
// this is only used for prompting passphrase secrets managers to show
// the stackname in the prompt for the passphrase.
type NamedStackProvider = secrets.NamedStackProvider

// DefaultProvider is the default DefaultProvider to use when deserializing deployments.
var DefaultProvider = secrets.DefaultProvider

