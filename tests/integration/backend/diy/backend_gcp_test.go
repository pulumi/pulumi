package diy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // this test sets the global login state
func TestGcpLogin(t *testing.T) {
	err := os.Chdir("project")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Chdir("..")
		require.NoError(t, err)
	})

	if _, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); !ok {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS not set, skipping test")
	}

	cloudURL := "gs://pulumitesting"
	loginAndCreateStack(t, cloudURL)
}
