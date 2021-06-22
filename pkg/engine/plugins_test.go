// Copyright 2016-2019, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func mustMakeVersion(v string) *semver.Version {
	ver := semver.MustParse(v)
	return &ver
}

func TestDefaultProvidersSingle(t *testing.T) {
	languagePlugins := newPluginSet()
	languagePlugins.Add(workspace.PluginInfo{
		Name:    "aws",
		Version: mustMakeVersion("0.17.1"),
		Kind:    workspace.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginInfo{
		Name:    "kubernetes",
		Version: mustMakeVersion("0.22.0"),
		Kind:    workspace.ResourcePlugin,
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, newPluginSet())
	assert.NotNil(t, defaultProviders)

	awsVer, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.1", awsVer.String())

	kubernetesVer, ok := defaultProviders[tokens.Package("kubernetes")]
	assert.True(t, ok)
	assert.NotNil(t, kubernetesVer)
	assert.Equal(t, "0.22.0", kubernetesVer.String())

}

func TestDefaultProvidersOverrideNoVersion(t *testing.T) {
	languagePlugins := newPluginSet()
	languagePlugins.Add(workspace.PluginInfo{
		Name:    "aws",
		Version: mustMakeVersion("0.17.1"),
		Kind:    workspace.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginInfo{
		Name:    "aws",
		Version: nil,
		Kind:    workspace.ResourcePlugin,
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, newPluginSet())
	assert.NotNil(t, defaultProviders)
	awsVer, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.1", awsVer.String())
}

func TestDefaultProvidersOverrideNewerVersion(t *testing.T) {
	languagePlugins := newPluginSet()
	languagePlugins.Add(workspace.PluginInfo{
		Name:    "aws",
		Version: mustMakeVersion("0.17.0"),
		Kind:    workspace.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginInfo{
		Name:    "aws",
		Version: mustMakeVersion("0.17.1"),
		Kind:    workspace.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginInfo{
		Name:    "aws",
		Version: mustMakeVersion("0.17.2-dev.1553126336"),
		Kind:    workspace.ResourcePlugin,
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, newPluginSet())
	assert.NotNil(t, defaultProviders)
	awsVer, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.2-dev.1553126336", awsVer.String())
}

func TestDefaultProvidersSnapshotOverrides(t *testing.T) {
	languagePlugins := newPluginSet()
	languagePlugins.Add(workspace.PluginInfo{
		Name: "python",
		Kind: workspace.LanguagePlugin,
	})
	snapshotPlugins := newPluginSet()
	snapshotPlugins.Add(workspace.PluginInfo{
		Name:    "aws",
		Version: mustMakeVersion("0.17.0"),
		Kind:    workspace.ResourcePlugin,
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, snapshotPlugins)
	assert.NotNil(t, defaultProviders)
	awsVer, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.0", awsVer.String())
}
