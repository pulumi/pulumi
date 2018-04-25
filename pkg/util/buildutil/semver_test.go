package buildutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersions(t *testing.T) {
	cases := map[string]string{
		"v0.12.0":                                "0.12.0",
		"v0.12.0-dirty":                          "0.12.0+dirty",
		"v0.12.0-1524606809-gf2f1178b":           "0.12.0.post1524606809+gf2f1178b",
		"v0.12.0-1524606809-gf2f1178b-dirty":     "0.12.0.post1524606809+gf2f1178b.dirty",
		"v0.12.0-rc1":                            "0.12.0rc1",
		"v0.12.0-rc1-1524606809-gf2f1178b":       "0.12.0rc1.post1524606809+gf2f1178b",
		"v0.12.0-rc1-1524606809-gf2f1178b-dirty": "0.12.0rc1.post1524606809+gf2f1178b.dirty",
		"v0.12.1-dev-1524606809-gf2f1178b":       "0.12.1.dev1524606809+gf2f1178b",
		"v0.12.1-dev-1524606809-gf2f1178b-dirty": "0.12.1.dev1524606809+gf2f1178b.dirty",
	}

	for ver, expected := range cases {
		p, err := PyPiVersionFromNpmVersion(ver)
		assert.NoError(t, err)
		assert.Equal(t, expected, p, "failed parsing '%s'", ver)
	}
}
