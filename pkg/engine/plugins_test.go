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
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func mustMakeVersion(v string) *semver.Version {
	ver := semver.MustParse(v)
	return &ver
}

func TestDefaultProvidersSingle(t *testing.T) {
	t.Parallel()

	languagePlugins := NewPluginSet()
	languagePlugins.Add(workspace.PluginSpec{
		Name:    "aws",
		Version: mustMakeVersion("0.17.1"),
		Kind:    apitype.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginSpec{
		Name:              "kubernetes",
		Version:           mustMakeVersion("0.22.0"),
		Kind:              apitype.ResourcePlugin,
		PluginDownloadURL: "com.server.url",
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, NewPluginSet())
	assert.NotNil(t, defaultProviders)

	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.1", awsVer.String())

	kubernetes, ok := defaultProviders[tokens.Package("kubernetes")]
	assert.True(t, ok)
	kubernetesVer := kubernetes.Version
	assert.NotNil(t, kubernetesVer)
	assert.Equal(t, "0.22.0", kubernetesVer.String())
	assert.Equal(t, "com.server.url", kubernetes.PluginDownloadURL)
}

func TestDefaultProvidersOverrideNoVersion(t *testing.T) {
	t.Parallel()

	languagePlugins := NewPluginSet()
	languagePlugins.Add(workspace.PluginSpec{
		Name:    "aws",
		Version: mustMakeVersion("0.17.1"),
		Kind:    apitype.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginSpec{
		Name:    "aws",
		Version: nil,
		Kind:    apitype.ResourcePlugin,
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, NewPluginSet())
	assert.NotNil(t, defaultProviders)
	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.1", awsVer.String())
}

func TestDefaultProvidersOverrideNewerVersion(t *testing.T) {
	t.Parallel()

	languagePlugins := NewPluginSet()
	languagePlugins.Add(workspace.PluginSpec{
		Name:    "aws",
		Version: mustMakeVersion("0.17.0"),
		Kind:    apitype.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginSpec{
		Name:    "aws",
		Version: mustMakeVersion("0.17.1"),
		Kind:    apitype.ResourcePlugin,
	})
	languagePlugins.Add(workspace.PluginSpec{
		Name:    "aws",
		Version: mustMakeVersion("0.17.2-dev.1553126336"),
		Kind:    apitype.ResourcePlugin,
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, NewPluginSet())
	assert.NotNil(t, defaultProviders)
	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.2-dev.1553126336", awsVer.String())
}

func TestDefaultProvidersSnapshotOverrides(t *testing.T) {
	t.Parallel()

	languagePlugins := NewPluginSet()
	languagePlugins.Add(workspace.PluginSpec{
		Name: "python",
		Kind: apitype.LanguagePlugin,
	})
	snapshotPlugins := NewPluginSet()
	snapshotPlugins.Add(workspace.PluginSpec{
		Name:    "aws",
		Version: mustMakeVersion("0.17.0"),
		Kind:    apitype.ResourcePlugin,
	})

	defaultProviders := computeDefaultProviderPlugins(languagePlugins, snapshotPlugins)
	assert.NotNil(t, defaultProviders)
	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	assert.NotNil(t, awsVer)
	assert.Equal(t, "0.17.0", awsVer.String())
}

func TestPluginSetDeduplicate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input    PluginSet
		expected PluginSet
	}{{
		input: NewPluginSet(workspace.PluginSpec{
			Name:    "foo",
			Version: &semver.Version{Major: 1},
		}, workspace.PluginSpec{
			Name: "foo",
		}),
		expected: NewPluginSet(workspace.PluginSpec{
			Name:    "foo",
			Version: &semver.Version{Major: 1},
		}),
	}, {
		input: NewPluginSet(workspace.PluginSpec{
			Name:    "bar",
			Version: &semver.Version{Minor: 3},
		}, workspace.PluginSpec{
			Name:              "bar",
			PluginDownloadURL: "example.com/bar",
		}, workspace.PluginSpec{
			Name:              "bar",
			Version:           &semver.Version{Patch: 3},
			PluginDownloadURL: "example.com",
		}, workspace.PluginSpec{
			Name: "foo",
		}),
		expected: NewPluginSet(workspace.PluginSpec{
			Name:    "bar",
			Version: &semver.Version{Minor: 3},
		}, workspace.PluginSpec{
			Name:              "bar",
			PluginDownloadURL: "example.com/bar",
		}, workspace.PluginSpec{
			Name:              "bar",
			Version:           &semver.Version{Patch: 3},
			PluginDownloadURL: "example.com",
		}, workspace.PluginSpec{
			Name: "foo",
		}),
	}}

	for _, c := range cases { //nolint:parallelTest
		c := c
		t.Run(fmt.Sprintf("%s", c.input), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expected, c.input.Deduplicate())
		})
	}
}

func TestPluginSetUpdatesTo(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name     string
		olds     PluginSet
		news     PluginSet
		expected []PluginUpdate
	}{
		{
			name:     "Both empty",
			olds:     NewPluginSet(),
			news:     NewPluginSet(),
			expected: nil,
		},
		{
			name: "Olds empty",
			olds: NewPluginSet(),
			news: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Version: &semver.Version{Major: 1},
			}),
			expected: nil,
		},
		{
			name: "News empty",
			olds: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Version: &semver.Version{Major: 1},
			}),
			news:     NewPluginSet(),
			expected: nil,
		},
		{
			name: "No matches by name",
			olds: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Version: &semver.Version{Major: 1},
			}),
			news: NewPluginSet(workspace.PluginSpec{
				Name:    "bar",
				Version: &semver.Version{Major: 1},
			}),
			expected: nil,
		},
		{
			name: "No matches by kind",
			olds: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Kind:    apitype.ResourcePlugin,
				Version: &semver.Version{Major: 1},
			}),
			news: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Kind:    apitype.AnalyzerPlugin,
				Version: &semver.Version{Major: 1},
			}),
			expected: nil,
		},
		{
			name: "Matches with no updates (equal)",
			olds: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Kind:    apitype.ResourcePlugin,
				Version: &semver.Version{Major: 1},
			}),
			news: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Kind:    apitype.ResourcePlugin,
				Version: &semver.Version{Major: 1},
			}),
			expected: nil,
		},
		{
			name: "Matches with no updates (news has an older version)",
			olds: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Kind:    apitype.ResourcePlugin,
				Version: &semver.Version{Major: 2},
			}),
			news: NewPluginSet(workspace.PluginSpec{
				Name:    "foo",
				Kind:    apitype.ResourcePlugin,
				Version: &semver.Version{Major: 1},
			}),
			expected: nil,
		},
		{
			name: "Matches with one update",
			olds: NewPluginSet(
				workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
				workspace.PluginSpec{
					Name:    "bar",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			),
			news: NewPluginSet(
				workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 2},
				},
			),
			expected: []PluginUpdate{
				{
					Old: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 1},
					},
					New: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 2},
					},
				},
			},
		},
		{
			name: "Matches with multiple updates",
			olds: NewPluginSet(
				workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
				workspace.PluginSpec{
					Name:    "bar",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 2},
				},
				workspace.PluginSpec{
					Name:    "baz",
					Kind:    apitype.AnalyzerPlugin,
					Version: &semver.Version{Major: 3},
				},
			),
			news: NewPluginSet(
				workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 2},
				},
				workspace.PluginSpec{
					Name:    "bar",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 2},
				},
				workspace.PluginSpec{
					Name:    "baz",
					Kind:    apitype.AnalyzerPlugin,
					Version: &semver.Version{Major: 4},
				},
			),
			expected: []PluginUpdate{
				{
					Old: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 1},
					},
					New: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 2},
					},
				},
				{
					Old: workspace.PluginSpec{
						Name:    "baz",
						Kind:    apitype.AnalyzerPlugin,
						Version: &semver.Version{Major: 3},
					},
					New: workspace.PluginSpec{
						Name:    "baz",
						Kind:    apitype.AnalyzerPlugin,
						Version: &semver.Version{Major: 4},
					},
				},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// Act.
			actual := c.news.UpdatesTo(c.olds)

			// Assert.
			require.ElementsMatch(t, c.expected, actual)
		})
	}
}

func TestDefaultProviderPluginsSorting(t *testing.T) {
	t.Parallel()
	v1 := semver.MustParse("0.0.1-alpha.10")
	p1 := workspace.PluginSpec{
		Name:    "foo",
		Version: &v1,
		Kind:    apitype.ResourcePlugin,
	}
	v2 := semver.MustParse("0.0.1-alpha.10+dirty")
	p2 := workspace.PluginSpec{
		Name:    "foo",
		Version: &v2,
		Kind:    apitype.ResourcePlugin,
	}
	plugins := NewPluginSet(p1, p2)
	result := computeDefaultProviderPlugins(plugins, plugins)
	assert.Equal(t, map[tokens.Package]workspace.PluginSpec{
		"foo": p2,
	}, result)
}
