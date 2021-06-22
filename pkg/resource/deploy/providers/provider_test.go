package providers

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestProviderRequestNameNil(t *testing.T) {
	req := NewProviderRequest(nil, "pkg")
	assert.Equal(t, tokens.QName("default"), req.Name())
	assert.Equal(t, "pkg", req.String())
}

func TestProviderRequestNameNoPre(t *testing.T) {
	ver := semver.MustParse("0.18.1")
	req := NewProviderRequest(&ver, "pkg")
	assert.Equal(t, "default_0_18_1", req.Name().String())
	assert.Equal(t, "pkg-0.18.1", req.String())
}

func TestProviderRequestNameDev(t *testing.T) {
	ver := semver.MustParse("0.17.7-dev.1555435978+gb7030aa4.dirty")
	req := NewProviderRequest(&ver, "pkg")
	assert.Equal(t, "default_0_17_7_dev_1555435978_gb7030aa4_dirty", req.Name().String())
	assert.Equal(t, "pkg-0.17.7-dev.1555435978+gb7030aa4.dirty", req.String())
}
