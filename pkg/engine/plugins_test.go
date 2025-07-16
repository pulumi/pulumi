// Copyright 2016-2024, Pulumi Corporation.
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

	languagePlugins := NewPackageSet()
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "aws",
			Version: mustMakeVersion("0.17.1"),
			Kind:    apitype.ResourcePlugin,
		},
	})
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:              "kubernetes",
			Version:           mustMakeVersion("0.22.0"),
			Kind:              apitype.ResourcePlugin,
			PluginDownloadURL: "com.server.url",
		},
	})

	defaultProviders := computeDefaultProviderPackages(languagePlugins, NewPackageSet())
	require.NotNil(t, defaultProviders)

	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	require.NotNil(t, awsVer)
	assert.Equal(t, "0.17.1", awsVer.String())

	kubernetes, ok := defaultProviders[tokens.Package("kubernetes")]
	assert.True(t, ok)
	kubernetesVer := kubernetes.Version
	require.NotNil(t, kubernetesVer)
	assert.Equal(t, "0.22.0", kubernetesVer.String())
	assert.Equal(t, "com.server.url", kubernetes.PluginDownloadURL)
}

func TestDefaultProvidersOverrideNoVersion(t *testing.T) {
	t.Parallel()

	languagePlugins := NewPackageSet()
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "aws",
			Version: mustMakeVersion("0.17.1"),
			Kind:    apitype.ResourcePlugin,
		},
	})
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "aws",
			Version: nil,
			Kind:    apitype.ResourcePlugin,
		},
	})

	defaultProviders := computeDefaultProviderPackages(languagePlugins, NewPackageSet())
	require.NotNil(t, defaultProviders)
	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	require.NotNil(t, awsVer)
	assert.Equal(t, "0.17.1", awsVer.String())
}

func TestDefaultProvidersOverrideNewerVersion(t *testing.T) {
	t.Parallel()

	languagePlugins := NewPackageSet()
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "aws",
			Version: mustMakeVersion("0.17.0"),
			Kind:    apitype.ResourcePlugin,
		},
	})
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "aws",
			Version: mustMakeVersion("0.17.1"),
			Kind:    apitype.ResourcePlugin,
		},
	})
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "aws",
			Version: mustMakeVersion("0.17.2-dev.1553126336"),
			Kind:    apitype.ResourcePlugin,
		},
	})

	defaultProviders := computeDefaultProviderPackages(languagePlugins, NewPackageSet())
	require.NotNil(t, defaultProviders)
	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	require.NotNil(t, awsVer)
	assert.Equal(t, "0.17.2-dev.1553126336", awsVer.String())
}

func TestDefaultProvidersSnapshotOverrides(t *testing.T) {
	t.Parallel()

	languagePlugins := NewPackageSet()
	languagePlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name: "python",
			Kind: apitype.LanguagePlugin,
		},
	})
	snapshotPlugins := NewPackageSet()
	snapshotPlugins.Add(workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "aws",
			Version: mustMakeVersion("0.17.0"),
			Kind:    apitype.ResourcePlugin,
		},
	})

	defaultProviders := computeDefaultProviderPackages(languagePlugins, snapshotPlugins)
	require.NotNil(t, defaultProviders)
	aws, ok := defaultProviders[tokens.Package("aws")]
	assert.True(t, ok)
	awsVer := aws.Version
	require.NotNil(t, awsVer)
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

func TestPackageSetUpdatesTo(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name     string
		olds     PackageSet
		news     PackageSet
		expected []PackageUpdate
	}{
		{
			name:     "Both empty",
			olds:     NewPackageSet(),
			news:     NewPackageSet(),
			expected: nil,
		},
		{
			name: "Olds empty",
			olds: NewPackageSet(),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			expected: nil,
		},
		{
			name: "News empty",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			news:     NewPackageSet(),
			expected: nil,
		},
		{
			name: "No matches by name",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "bar",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			expected: nil,
		},
		{
			name: "No matches by kind",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.AnalyzerPlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			expected: nil,
		},
		{
			name: "Matches with no updates (equal)",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			expected: nil,
		},
		{
			name: "Matches with no updates (news has an older version)",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 2},
				},
			}),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			expected: nil,
		},
		{
			name: "Matches with one update",
			olds: NewPackageSet(
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 1},
					},
				},
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "bar",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 1},
					},
				},
			),
			news: NewPackageSet(
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 2},
					},
				},
			),
			expected: []PackageUpdate{
				{
					Old: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "foo",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 1},
						},
					},
					New: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "foo",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 2},
						},
					},
				},
			},
		},
		{
			name: "Matches with multiple updates",
			olds: NewPackageSet(
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 1},
					},
				},
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "bar",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 2},
					},
				},
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "baz",
						Kind:    apitype.AnalyzerPlugin,
						Version: &semver.Version{Major: 3},
					},
				},
			),
			news: NewPackageSet(
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "foo",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 2},
					},
				},
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "bar",
						Kind:    apitype.ResourcePlugin,
						Version: &semver.Version{Major: 2},
					},
				},
				workspace.PackageDescriptor{
					PluginSpec: workspace.PluginSpec{
						Name:    "baz",
						Kind:    apitype.AnalyzerPlugin,
						Version: &semver.Version{Major: 4},
					},
				},
			),
			expected: []PackageUpdate{
				{
					Old: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "foo",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 1},
						},
					},

					New: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "foo",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 2},
						},
					},
				},
				{
					Old: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "baz",
							Kind:    apitype.AnalyzerPlugin,
							Version: &semver.Version{Major: 3},
						},
					},
					New: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "baz",
							Kind:    apitype.AnalyzerPlugin,
							Version: &semver.Version{Major: 4},
						},
					},
				},
			},
		},
		{
			name: "Base plugin and parameterized package",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 2},
				},
				Parameterization: &workspace.Parameterization{
					Name:    "bar",
					Version: semver.Version{Major: 2},
					Value:   []byte("data"),
				},
			}),
			expected: nil,
		},
		{
			name: "Parameterized package update",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
				Parameterization: &workspace.Parameterization{
					Name:    "bar",
					Version: semver.Version{Major: 1},
					Value:   []byte("data"),
				},
			}),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
				Parameterization: &workspace.Parameterization{
					Name:    "bar",
					Version: semver.Version{Major: 2},
					Value:   []byte("data"),
				},
			}),
			expected: []PackageUpdate{
				{
					Old: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "foo",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 1},
						},
						Parameterization: &workspace.Parameterization{
							Name:    "bar",
							Version: semver.Version{Major: 1},
							Value:   []byte("data"),
						},
					},
					New: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "foo",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 1},
						},
						Parameterization: &workspace.Parameterization{
							Name:    "bar",
							Version: semver.Version{Major: 2},
							Value:   []byte("data"),
						},
					},
				},
			},
		},
		{
			name: "Non-parameterized to parameterized package update",
			olds: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "foo",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
			}),
			news: NewPackageSet(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "base",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1},
				},
				Parameterization: &workspace.Parameterization{
					Name:    "foo",
					Version: semver.Version{Major: 2},
					Value:   []byte("data"),
				},
			}),
			expected: []PackageUpdate{
				{
					Old: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "foo",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 1},
						},
					},
					New: workspace.PackageDescriptor{
						PluginSpec: workspace.PluginSpec{
							Name:    "base",
							Kind:    apitype.ResourcePlugin,
							Version: &semver.Version{Major: 1},
						},
						Parameterization: &workspace.Parameterization{
							Name:    "foo",
							Version: semver.Version{Major: 2},
							Value:   []byte("data"),
						},
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
	p1 := workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "foo",
			Version: &v1,
			Kind:    apitype.ResourcePlugin,
		},
	}
	v2 := semver.MustParse("0.0.1-alpha.10+dirty")
	p2 := workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:    "foo",
			Version: &v2,
			Kind:    apitype.ResourcePlugin,
		},
	}
	plugins := NewPackageSet(p1, p2)
	result := computeDefaultProviderPackages(plugins, plugins)
	assert.Equal(t, map[tokens.Package]workspace.PackageDescriptor{
		"foo": p2,
	}, result)
}
