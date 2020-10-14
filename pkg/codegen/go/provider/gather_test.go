package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGatherSinglePackage(t *testing.T) {
	rootPath, packages, err := loadRandomPackage()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Len(t, packages, 1)

	pp, diags := gatherPulumiPackage("myrandom", rootPath, packages)
	assert.False(t, diags.HasErrors())

	assert.NotNil(t, pp.provider)

	assert.Len(t, pp.modules, 1)
	index := pp.modules[0]

	assert.Equal(t, "index", index.name)

	assert.Len(t, index.resources, 1)
	assert.Equal(t, index.resources[0].syntax.Name.Name, "RandomBytes")

	assert.Len(t, index.functions, 1)
	assert.Equal(t, index.functions[0].syntax.Name.Name, "GetRandomBytes")

	assert.Len(t, index.constructors, 1)
	assert.Equal(t, index.constructors[0].syntax.Name.Name, "NewHashComponent")
}

func TestGatherPackageTree(t *testing.T) {
	rootPath, packages, err := loadLandscapePackage()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Len(t, packages, 4)

	pp, diags := gatherPulumiPackage("landscape", rootPath, packages)
	assert.False(t, diags.HasErrors())

	assert.NotNil(t, pp.provider)

	assert.Len(t, pp.modules, 4)

	for _, m := range pp.modules {
		assert.Len(t, m.functions, 0)
		assert.Len(t, m.constructors, 0)

		var expected []string
		switch m.name {
		case "index":
			expected = nil
		case "trees":
			expected = []string{"CedarTree", "MapleTree", "OakTree"}
		case "ferns":
			expected = []string{"DeerFern", "SwordFern"}
		case "containers":
			expected = []string{"ClayPot"}
		default:
			assert.Fail(t, "unexpected pacakge %v", m.name)
			continue
		}

		names := make([]string, len(m.resources))
		for i, r := range m.resources {
			names[i] = r.syntax.Name.Name
		}
		assert.Subset(t, expected, names)
	}
}
