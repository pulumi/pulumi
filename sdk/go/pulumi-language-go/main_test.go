package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var pluginTests = []struct {
	Description string
	Mod         modInfo
	Name        string
	Version     string
	ShouldError bool
}{
	{
		Description: "validPulumiMod",
		Mod: modInfo{
			Path:    "github.com/pulumi/pulumi-aws/sdk",
			Version: "v1.29.0",
		},
		Name: "aws",
	},
	{
		Description: "pulumiPseudoVersionPlugin",
		Mod: modInfo{
			Path:    "github.com/pulumi/pulumi-aws/sdk",
			Version: "v1.29.1-0.20200403140640-efb5e2a48a86",
		},
		Name:    "aws",
		Version: "v1.29.0",
	},
	{
		Description: "nonPulumi",
		Mod: modInfo{
			Path:    "github.com/moolumi/pulumi-aws/sdk",
			Version: "v1.29.0",
		},
		Name: "aws",
	},
	{
		Description: "invalidVersionModule",
		Mod: modInfo{
			Path:    "github.com/pulumi/pulumi-aws/sdk",
			Version: "42-42-42",
		}, ShouldError: true,
	},
	{
		Description: "pulumiPulumiModule",
		Mod: modInfo{
			Path:    "github.com/pulumi/pulumi/sdk",
			Version: "v1.14.0",
		},
		ShouldError: true,
	},
	{
		Description: "betaModule",
		Mod: modInfo{
			Path:    "github.com/pulumi/pulumi-aws/sdk",
			Version: "v2.0.0-beta.1",
		},
		Name: "aws",
	},
	{
		Description: "nonZeroPatchModule",
		Mod: modInfo{
			Path:    "github.com/pulumi/pulumi-kubernetes/sdk",
			Version: "v1.5.8",
		},
		Name: "kubernetes",
	},
	{
		Description: "nonGithubName",
		Mod: modInfo{
			Path:    "sourcegraph.com/sourcegraph/pulumi-appdash",
			Version: "v2.3.4",
		},
		Name: "appdash",
	},
	{
		Description: "eagerPath",
		Mod: modInfo{
			Path:    "mysite.com/pulumi-foo",
			Version: "v1.2.3",
		},
		ShouldError: true,
	},
	{
		Description: "lazyPath",
		Mod: modInfo{
			Path:    "mysite.com/foo/bar/pulumi-fubar",
			Version: "v0.0.1",
		},
		ShouldError: true,
	},
	{
		Description: "hyphenatedPath",
		Mod: modInfo{
			Path:    "github.com/my-name/pulumi-hello-world",
			Version: "v0.0.0",
		},
		Name: "hello-world",
	},
}

func TestGetPlugin(t *testing.T) {
	for _, tt := range pluginTests {
		t.Run(tt.Description, func(t *testing.T) {
			plugin, err := tt.Mod.getPlugin()
			if !tt.ShouldError {
				if tt.Version == "" {
					tt.Version = tt.Mod.Version
				}
				assert.NoError(t, err)
				assert.Equal(t, tt.Name, plugin.Name)
				assert.Equal(t, tt.Version, plugin.Version)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
