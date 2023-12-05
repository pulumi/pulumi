package providers

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

func TestProviderRequestNameNil(t *testing.T) {
	t.Parallel()

	req := NewProviderRequest(nil, "pkg", "", nil)
	assert.Equal(t, "default", req.Name())
	assert.Equal(t, "pkg", req.String())
}

func TestProviderRequestNameNoPre(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.18.1")
	req := NewProviderRequest(&ver, "pkg", "", nil)
	assert.Equal(t, "default_0_18_1", req.Name())
	assert.Equal(t, "pkg-0.18.1", req.String())
}

func TestProviderRequestNameDev(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.17.7-dev.1555435978+gb7030aa4.dirty")
	req := NewProviderRequest(&ver, "pkg", "", nil)
	assert.Equal(t, "default_0_17_7_dev_1555435978_gb7030aa4_dirty", req.Name())
	assert.Equal(t, "pkg-0.17.7-dev.1555435978+gb7030aa4.dirty", req.String())
}

func TestProviderRequestNameNoPreURL(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.18.1")
	req := NewProviderRequest(&ver, "pkg", "pulumi.com/pkg", nil)
	assert.Equal(t, "default_0_18_1_pulumi.com/pkg", req.Name())
	assert.Equal(t, "pkg-0.18.1-pulumi.com/pkg", req.String())
}

func TestProviderRequestNameDevURL(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.17.7-dev.1555435978+gb7030aa4.dirty")
	req := NewProviderRequest(&ver, "pkg", "company.com/artifact-storage/pkg", nil)
	assert.Equal(t, "default_0_17_7_dev_1555435978_gb7030aa4_dirty_company.com/artifact-storage/pkg", req.Name())
	assert.Equal(t, "pkg-0.17.7-dev.1555435978+gb7030aa4.dirty-company.com/artifact-storage/pkg", req.String())
}

func TestProviderRequestCanonicalizeURL(t *testing.T) {
	t.Parallel()

	req := NewProviderRequest(nil, "pkg", "company.com/", nil)
	assert.Equal(t, "company.com", req.PluginDownloadURL())
	assert.Equal(t, "default_company.com", req.Name())
}
