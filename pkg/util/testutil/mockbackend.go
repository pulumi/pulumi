package testutil

import testutil "github.com/pulumi/pulumi/sdk/v3/pkg/util/testutil"

// MockBackendInstance sets the backend instance for the test and cleans it up after.
func MockBackendInstance(t *testing.T, b backend.Backend) {
	testutil.MockBackendInstance(t, b)
}

// MockLoginManager sets the login manager for the test and cleans it up after.
func MockLoginManager(t *testing.T, lm cmdBackend.LoginManager) {
	testutil.MockLoginManager(t, lm)
}

