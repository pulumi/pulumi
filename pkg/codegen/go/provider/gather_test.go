package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGather(t *testing.T) {
	packages, err := loadRandomPackage()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Len(t, packages, 1)

	pp, diags := gatherPulumiPackage("myrandom", packages)
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
