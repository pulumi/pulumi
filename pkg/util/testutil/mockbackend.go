package testutil

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
)

// MockBackendInstance sets the backend instance for the test and cleans it up after.
func MockBackendInstance(t *testing.T, b backend.Backend) {
	t.Cleanup(func() {
		cmdBackend.BackendInstance = nil
	})
	cmdBackend.BackendInstance = b
}

// MockLoginManager sets the login manager for the test and cleans it up after.
func MockLoginManager(t *testing.T, lm cmdBackend.LoginManager) {
	t.Cleanup(func() {
		cmdBackend.DefaultLoginManager = nil
	})
	cmdBackend.DefaultLoginManager = lm
}
