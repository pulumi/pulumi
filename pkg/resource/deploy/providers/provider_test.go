package providers

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

func TestProviderRequestNameNil(t *testing.T) {
	t.Parallel()

	req := NewProviderRequest("pkg", nil, "", nil, nil)
	assert.Equal(t, "default", req.DefaultName())
	assert.Equal(t, "pkg", req.String())
}

func TestProviderRequestNameNoPre(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.18.1")
	req := NewProviderRequest("pkg", &ver, "", nil, nil)
	assert.Equal(t, "default_0_18_1", req.DefaultName())
	assert.Equal(t, "pkg-0.18.1", req.String())
}

func TestProviderRequestNameDev(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.17.7-dev.1555435978+gb7030aa4.dirty")
	req := NewProviderRequest("pkg", &ver, "", nil, nil)
	assert.Equal(t, "default_0_17_7_dev_1555435978_gb7030aa4_dirty", req.DefaultName())
	assert.Equal(t, "pkg-0.17.7-dev.1555435978+gb7030aa4.dirty", req.String())
}

func TestProviderRequestNameNoPreURL(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.18.1")
	req := NewProviderRequest("pkg", &ver, "pulumi.com/pkg", nil, nil)
	assert.Equal(t, "default_0_18_1_pulumi.com/pkg", req.DefaultName())
	assert.Equal(t, "pkg-0.18.1-pulumi.com/pkg", req.String())
}

func TestProviderRequestNameDevURL(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("0.17.7-dev.1555435978+gb7030aa4.dirty")
	req := NewProviderRequest("pkg", &ver, "company.com/artifact-storage/pkg", nil, nil)
	assert.Equal(t, "default_0_17_7_dev_1555435978_gb7030aa4_dirty_company.com/artifact-storage/pkg", req.DefaultName())
	assert.Equal(t, "pkg-0.17.7-dev.1555435978+gb7030aa4.dirty-company.com/artifact-storage/pkg", req.String())
}

func TestProviderRequestCanonicalizeURL(t *testing.T) {
	t.Parallel()

	req := NewProviderRequest("pkg", nil, "company.com/", nil, nil)
	assert.Equal(t, "company.com", req.PluginDownloadURL())
	assert.Equal(t, "default_company.com", req.DefaultName())
}
