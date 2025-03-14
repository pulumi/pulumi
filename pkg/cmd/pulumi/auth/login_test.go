package auth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckHttpCloudBackendUrlWithAppPulumi calls checkHttpCloudBackenUrl with a
// wrong pulumi cloud url, checking
// for a valid return value.
func TestCheckHttpCloudBackendUrlWithAppPulumi(t *testing.T) {
	cloudUrl := "https://app.pulumi.com"
	want := "https://api.pulumi.com"
	url, err := checkHttpCloudBackenUrl(cloudUrl)

	assert.NoError(t, err)
	assert.Equal(t, want, url)
}

// TestCheckHttpCloudBackendUrlWithAppWhatever calls checkHttpCloudBackenUrl with a wrong url,
// checking for an error.
func TestCheckHttpCloudBackendUrlWithAppWhatever(t *testing.T) {
	cloudUrl := "https://app.pulumi.test.com"
	url, err := checkHttpCloudBackenUrl(cloudUrl)
	assert.Empty(t, url)
	if assert.Error(t, err) {
		assert.EqualError(t, err, fmt.Sprintf("did you mean %s ?", "https://api.pulumi.test.com"))
	}
}
