package main

import (
	"errors"
	"github.com/stretchr/testify/require"
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

func Test_checkMinimumGoVersion(t *testing.T) {
	tests := []struct {
		name            string
		goVersionOutput string
		err             error
	}{
		{
			name:            "ExactVersion",
			goVersionOutput: "go version go1.14.0 darwin/amd64",
		},
		{
			name:            "NewerVersion",
			goVersionOutput: "go version go1.15.1 darwin/amd64",
		},
		{
			name:            "OlderGoVersion",
			goVersionOutput: "go version go1.13.8 linux/amd64",
			err:             errors.New("go version must be 1.14.0 or higher (1.13.8 detected)"),
		},
		{
			name:            "MalformedVersion",
			goVersionOutput: "go version xyz",
			err:             errors.New("parsing go version failed: Invalid character(s) found in major number \"xyz\""),
		},
		{
			name:            "GarbageVersionOutput",
			goVersionOutput: "gobble gobble",
			err:             errors.New("unexpected format for go version output: \"gobble gobble\""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkMinimumGoVersion(tt.goVersionOutput)
			if err != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
