package filestate

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

	t.Setenv("GOOGLE_PROJECT", "pulumi-ci-gcp-provider")
	cloudURL := "gs://pulumitesting"
	loginAndCreateStack(t, cloudURL)
}
