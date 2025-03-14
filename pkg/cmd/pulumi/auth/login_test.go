package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckHTTPCloudBackendUrlWithAppPulumi calls checkHTTPCloudBackenUrl with a
// wrong pulumi cloud url, checking for a valid return value.
func TestCheckHTTPCloudBackendUrlWithAppPulumi(t *testing.T) {
	cloudUrl := "https://app.pulumi.com"
	want := "https://api.pulumi.com"
	url, err := checkHTTPCloudBackenUrl(cloudUrl)

	require.NoError(t, err)
	require.Equal(t, want, url)
}
