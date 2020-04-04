package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPlugin(t *testing.T) {
	validPulumiMod := &modInfo{
		Path:    "github.com/pulumi/pulumi-aws/sdk",
		Version: "v1.29.0",
	}
	validPlugin, err := validPulumiMod.getPlugin()
	assert.Nil(t, err)
	assert.Equal(t, validPlugin.Name, "aws")
	assert.Equal(t, validPlugin.Version, "v1.29.0")

	pulumiPinnedModule := &modInfo{
		Path:    "github.com/pulumi/pulumi-aws/sdk",
		Version: "v1.29.1-0.20200403140640-efb5e2a48a86",
	}
	pulumiPinnedPlugin, err := pulumiPinnedModule.getPlugin()
	assert.Nil(t, err)
	assert.Equal(t, pulumiPinnedPlugin.Name, "aws")
	assert.Equal(t, pulumiPinnedPlugin.Version, "v1.29.0")

	nonPulumiMod := &modInfo{
		Path:    "github.com/moolumi/pulumi-aws/sdk",
		Version: "v1.29.0",
	}
	_, err = nonPulumiMod.getPlugin()
	assert.NotNil(t, err)

	invalidVersionModule := &modInfo{
		Path:    "github.com/pulumi/pulumi-aws/sdk",
		Version: "42-42-42",
	}
	_, err = invalidVersionModule.getPlugin()
	assert.NotNil(t, err)

	pulumiPulumiMod := &modInfo{
		Path:    "github.com/pulumi/pulumi/sdk",
		Version: "v1.14.0",
	}
	_, err = pulumiPulumiMod.getPlugin()
	assert.NotNil(t, err)
}
