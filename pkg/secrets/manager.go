package secrets

import secrets "github.com/pulumi/pulumi/sdk/v3/pkg/secrets"

// Manager provides the interface for providing stack encryption.
type Manager = secrets.Manager

// AreCompatible returns true if the two Managers are of the same type and have the same state.
func AreCompatible(a, b Manager) bool {
	return secrets.AreCompatible(a, b)
}

