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

	pulumiPseudoVersionModule := &modInfo{
		Path:    "github.com/pulumi/pulumi-aws/sdk",
		Version: "v1.29.1-0.20200403140640-efb5e2a48a86",
	}
	pulumiPseduoVersionPlugin, err := pulumiPseudoVersionModule.getPlugin()
	assert.Nil(t, err)
	assert.Equal(t, pulumiPseduoVersionPlugin.Name, "aws")
	assert.Equal(t, pulumiPseduoVersionPlugin.Version, "v1.29.0")

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

	betaPulumiModule := &modInfo{
		Path:    "github.com/pulumi/pulumi-aws/sdk",
		Version: "v2.0.0-beta.1",
	}
	betaPulumiPlugin, err := betaPulumiModule.getPlugin()
	assert.Nil(t, err)
	assert.Equal(t, betaPulumiPlugin.Name, "aws")
	assert.Equal(t, betaPulumiPlugin.Version, "v2.0.0-beta.1")

	nonZeroPatchModule := &modInfo{
		Path:    "github.com/pulumi/pulumi-kubernetes/sdk",
		Version: "v1.5.8",
	}
	nonZeroPatchPlugin, err := nonZeroPatchModule.getPlugin()
	assert.Nil(t, err)
	assert.Equal(t, nonZeroPatchPlugin.Name, "kubernetes")
	assert.Equal(t, nonZeroPatchPlugin.Version, "v1.5.8")
}
