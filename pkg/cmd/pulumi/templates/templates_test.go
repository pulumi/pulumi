// Copyright 2016-2025, Pulumi Corporation.
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

package templates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const expectedRegistryFormatError = "Expected: registry://templates/source/publisher/name[@version], " +
	"source/publisher/name[@version], publisher/name[@version], or name[@version]"

//nolint:paralleltest // replaces global backend instance
func TestFilterOnName(t *testing.T) {
	template1 := &apitype.PulumiTemplateRemote{
		ProjectTemplate: apitype.ProjectTemplate{},
		Name:            "name1",
		SourceName:      "source1",
		TemplateURL:     "example.com/source1/name1",
	}

	testFilterOnName := func(t *testing.T, mapStore env.MapStore) {
		ctx := testContext(t)

		source := newImpl(ctx, "name1",
			ScopeAll, workspace.TemplateKindPulumiProject,
			templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
			env.NewEnv(mapStore),
		)

		template, err := source.Templates()
		require.NoError(t, err)
		require.Len(t, template, 1)
		assert.Equal(t, "name1", template[0].Name())
		assert.Equal(t, "", template[0].Description())
		assert.Equal(t, "", template[0].ProjectDescription())
		assert.Nil(t, template[0].Error())
	}

	t.Run("org-backed-templates", func(t *testing.T) {
		mockBackend := &backend.MockBackend{
			SupportsTemplatesF: func() bool { return true },
			CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
				return "doe", []string{"org1"}, &workspace.TokenInformation{}, nil
			},
			ListTemplatesF: func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
				assert.Equal(t, "org1", orgName)

				return apitype.ListOrgTemplatesResponse{
					Templates: map[string][]*apitype.PulumiTemplateRemote{
						"source1": {template1},
						"source2": {
							{
								ProjectTemplate: apitype.ProjectTemplate{},
								Name:            "name2",
								SourceName:      "source2",
								TemplateURL:     "example.com/source2/name1",
							},
						},
					},
					OrgHasTemplates: true,
				}, nil
			},
		}
		testutil.MockBackendInstance(t, mockBackend)

		testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
			CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool,
			) (backend.Backend, error) {
				return mockBackend, nil
			},
			LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
			) (backend.Backend, error) {
				return mockBackend, nil
			},
		})
		testFilterOnName(t, env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "true",
		})
	})

	t.Run("org-backed-templates", func(t *testing.T) {
		listTemplates := func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			assert.Nil(t, name)
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				if !yield(apitype.TemplateMetadata{
					Name:      "name1",
					Publisher: "publisher1",
					Source:    "source1",
				}, nil) {
					return
				}
				if !yield(apitype.TemplateMetadata{
					Name:      "name2",
					Publisher: "publisher2",
					Source:    "source2",
				}, nil) {
					return
				}
			}
		}
		mockBackend := &backend.MockBackend{
			GetReadOnlyCloudRegistryF: func() registry.Registry {
				return &backend.MockCloudRegistry{
					ListTemplatesF: listTemplates,
				}
			},
		}
		testutil.MockBackendInstance(t, mockBackend)

		testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
			CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool,
			) (backend.Backend, error) {
				return mockBackend, nil
			},
			LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
			) (backend.Backend, error) {
				return mockBackend, nil
			},
		})
		testFilterOnName(t, env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
			"PULUMI_EXPERIMENTAL":             "true",
		})
	})
}

//nolint:paralleltest // replaces global backend instance
func TestMultipleTemplateSources_OrgTemplates(t *testing.T) {
	ctx := testContext(t)

	template1 := &apitype.PulumiTemplateRemote{
		ProjectTemplate: apitype.ProjectTemplate{},
		Name:            "name1",
		SourceName:      "source1",
		TemplateURL:     "example.com/source1/name1",
	}

	template2 := &apitype.PulumiTemplateRemote{
		ProjectTemplate: apitype.ProjectTemplate{},
		Name:            "name2",
		SourceName:      "source2",
		TemplateURL:     "example.com/source2/name1",
	}

	mockBackend := &backend.MockBackend{
		SupportsTemplatesF: func() bool { return true },
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "doe", []string{"org1"}, &workspace.TokenInformation{}, nil
		},
		ListTemplatesF: func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
			assert.Equal(t, "org1", orgName)

			return apitype.ListOrgTemplatesResponse{
				Templates: map[string][]*apitype.PulumiTemplateRemote{
					"source1": {template1},
					"source2": {template2},
				},
				OrgHasTemplates: true,
			}, nil
		},
	}
	testutil.MockBackendInstance(t, mockBackend)

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	repoTemplateDir := t.TempDir()
	subdir := filepath.Join(repoTemplateDir, "sub")
	require.NoError(t, os.Mkdir(subdir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "Pulumi.yaml"), []byte(`name: template3
runtime: dotnet
description: An ASP.NET application running a simple container in a EKS Cluster
`), 0o600))
	repoTemplates := templateRepository(workspace.TemplateRepository{
		Root:         repoTemplateDir,
		SubDirectory: subdir,
	}, nil)

	source := newImpl(ctx, "",
		ScopeAll, workspace.TemplateKindPulumiProject,
		repoTemplates, env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "true",
		}),
	)

	template, err := source.Templates()
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]Template{
			workspaceTemplate{t: workspace.Template{
				Dir:                subdir,
				Name:               "sub",
				ProjectName:        "template3",
				ProjectDescription: "An ASP.NET application running a simple container in a EKS Cluster",
			}},
			orgTemplate{t: template1, org: "org1", source: source, backend: cmdBackend.BackendInstance},
			orgTemplate{t: template2, org: "org1", source: source, backend: cmdBackend.BackendInstance},
		},
		template)
}

//nolint:paralleltest // replaces global backend instance
func TestSurfaceListTemplateErrors_OrgTemplates(t *testing.T) {
	ctx := testContext(t)

	somethingWentWrong := errors.New("something went wrong")

	mockBackend := &backend.MockBackend{
		SupportsTemplatesF: func() bool { return true },
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "doe", []string{"org1"}, &workspace.TokenInformation{}, nil
		},
		ListTemplatesF: func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
			assert.Equal(t, "org1", orgName)
			return apitype.ListOrgTemplatesResponse{}, somethingWentWrong
		},
	}
	testutil.MockBackendInstance(t, mockBackend)

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	source := newImpl(ctx, "name1",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{"PULUMI_DISABLE_REGISTRY_RESOLVE": "true"}),
	)

	_, err := source.Templates()
	assert.ErrorIs(t, err, somethingWentWrong)
}

//nolint:paralleltest // replaces global backend instance
func TestSurfaceListTemplateErrors_RegistryTemplates(t *testing.T) {
	ctx := testContext(t)

	somethingWentWrong := errors.New("something went wrong")

	mockRegistry := &backend.MockCloudRegistry{
		ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				yield(apitype.TemplateMetadata{}, somethingWentWrong)
			}
		},
	}

	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry { return mockRegistry },
	}
	testutil.MockBackendInstance(t, mockBackend)

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	source := newImpl(ctx, "name1",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
			"PULUMI_EXPERIMENTAL":             "true",
		}),
	)

	_, err := source.Templates()
	assert.ErrorIs(t, err, somethingWentWrong)
}

//nolint:paralleltest // replaces global backend instance
func TestSurfaceOnEmptyError_OrgTemplates(t *testing.T) {
	ctx := testContext(t)

	mockBackend := &backend.MockBackend{
		SupportsTemplatesF: func() bool { return true },
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "doe", []string{"org1"}, &workspace.TokenInformation{}, nil
		},
		ListTemplatesF: func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
			assert.Equal(t, "org1", orgName)
			return apitype.ListOrgTemplatesResponse{}, nil
		},
	}
	testutil.MockBackendInstance(t, mockBackend)

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	source := newImpl(ctx, "name1",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "true",
		}),
	)

	_, err := source.Templates()
	var expected workspace.TemplateNotFoundError
	assert.ErrorAsf(t, err, &expected, "what's in %#v", source.errorOnEmpty)
}

//nolint:paralleltest // replaces global backend instance
func TestSurfaceOnEmptyError_RegistryTemplates(t *testing.T) {
	ctx := testContext(t)

	mockRegistry := &backend.MockCloudRegistry{
		ListTemplatesF: func(_ context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			return func(func(apitype.TemplateMetadata, error) bool) {}
		},
	}
	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return mockRegistry
		},
	}
	testutil.MockBackendInstance(t, mockBackend)

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	source := newImpl(ctx, "name1",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
			"PULUMI_EXPERIMENTAL":             "true",
		}),
	)

	_, err := source.Templates()
	var expected workspace.TemplateNotFoundError
	assert.ErrorAsf(t, err, &expected, "what's in %#v", source.errorOnEmpty)
}

//nolint:paralleltest // replaces global backend instance
func TestTemplateDownload_Org(t *testing.T) {
	ctx := testContext(t)

	template1 := &apitype.PulumiTemplateRemote{
		ProjectTemplate: apitype.ProjectTemplate{},
		Name:            "name1",
		SourceName:      "source1",
		TemplateURL:     "example.com/source1/name1",
	}

	pulumiYAML := `name: template1
runtime: dotnet
description: An ASP.NET application running a simple container in a EKS Cluster
`
	anotherFile := `This is another file`

	mockBackend := &backend.MockBackend{
		SupportsTemplatesF: func() bool { return true },
		CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
			return "doe", []string{"org1"}, &workspace.TokenInformation{}, nil
		},
		ListTemplatesF: func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
			assert.Equal(t, "org1", orgName)

			return apitype.ListOrgTemplatesResponse{
				Templates: map[string][]*apitype.PulumiTemplateRemote{
					"source1": {template1},
				},
				OrgHasTemplates: true,
			}, nil
		},
		DownloadTemplateF: func(_ context.Context, orgName, templateSource string) (backend.TarReaderCloser, error) {
			assert.Equal(t, "org1", orgName)
			assert.Equal(t, template1.TemplateURL, templateSource)

			return backend.MockTarReader{
				"Pulumi.yaml": backend.MockTarFile{Content: pulumiYAML},
				"other.txt":   backend.MockTarFile{Content: anotherFile},
			}, nil
		},
	}
	testutil.MockBackendInstance(t, mockBackend)

	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	})

	source := newImpl(ctx, "name1",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "true",
		}),
	)

	template, err := source.Templates()
	require.NoError(t, err)
	assert.Equal(t,
		[]Template{orgTemplate{t: template1, org: "org1", source: source, backend: cmdBackend.BackendInstance}},
		template)
	t.Cleanup(func() {
		require.NoError(t, source.Close())
	})

	wTemplate, err := template[0].Download(ctx)
	require.NoError(t, err)

	{ // Pulumi.yaml
		file, err := os.ReadFile(filepath.Join(wTemplate.Dir, "Pulumi.yaml"))
		require.NoError(t, err)
		assert.Equal(t, pulumiYAML, string(file))
	}
	{ // other.txt
		file, err := os.ReadFile(filepath.Join(wTemplate.Dir, "other.txt"))
		require.NoError(t, err)
		assert.Equal(t, anotherFile, string(file))
	}
}

// Test data for registry template tests
var (
	testPulumiYAML = `name: template1
runtime: dotnet
description: An ASP.NET application running a simple container in a EKS Cluster
`
	testAnotherFile = `This is another file`
)

// createTestTarData creates tar archive data with test files
func createTestTarData(t *testing.T) []byte {
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)

	// Write Pulumi.yaml
	require.NoError(t, w.WriteHeader(&tar.Header{
		Name: "Pulumi.yaml",
		Size: int64(len(testPulumiYAML)),
		Mode: 0o600,
	}))
	_, err := w.Write([]byte(testPulumiYAML))
	require.NoError(t, err)

	// Write other.txt
	require.NoError(t, w.WriteHeader(&tar.Header{
		Name: "other.txt",
		Size: int64(len(testAnotherFile)),
		Mode: 0o600,
	}))
	_, err = w.Write([]byte(testAnotherFile))
	require.NoError(t, err)

	require.NoError(t, w.Close())
	return buf.Bytes()
}

func createMockRegistrySource(
	ctx context.Context,
	t *testing.T,
	downloadFunc func(context.Context, string) (io.ReadCloser, error),
) *Source {
	mockRegistry := &backend.MockCloudRegistry{
		ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				yield(apitype.TemplateMetadata{
					Name:        "name1",
					DownloadURL: "example.com/download/name",
				}, nil)
			}
		},
		DownloadTemplateF: downloadFunc,
	}
	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry { return mockRegistry },
	}
	testutil.MockBackendInstance(t, mockBackend)
	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{ /* panic on use */ })

	return newImpl(ctx, "name1",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
			"PULUMI_EXPERIMENTAL":             "true",
		}))
}

// testTemplateDownload tests downloading and verifying template content
func testTemplateDownload(ctx context.Context, t *testing.T, source *Source) {
	templates, err := source.Templates()
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "name1", templates[0].Name())
	t.Cleanup(func() {
		require.NoError(t, source.Close())
	})

	wTemplate, err := templates[0].Download(ctx)
	require.NoError(t, err)

	// Verify extracted files
	{ // Pulumi.yaml
		file, err := os.ReadFile(filepath.Join(wTemplate.Dir, "Pulumi.yaml"))
		require.NoError(t, err)
		assert.Equal(t, testPulumiYAML, string(file))
	}
	{ // other.txt
		file, err := os.ReadFile(filepath.Join(wTemplate.Dir, "other.txt"))
		require.NoError(t, err)
		assert.Equal(t, testAnotherFile, string(file))
	}
}

//nolint:paralleltest // replaces global backend instance
func TestTemplateDownload_Registry(t *testing.T) {
	ctx := testContext(t)

	source := createMockRegistrySource(ctx, t, func(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
		assert.Equal(t, "example.com/download/name", downloadURL)
		tarData := createTestTarData(t)
		return io.NopCloser(bytes.NewReader(tarData)), nil
	})

	testTemplateDownload(ctx, t, source)
}

//nolint:paralleltest // replaces global backend instance
func TestTemplateDownload_Registry_Gzipped(t *testing.T) {
	ctx := testContext(t)

	source := createMockRegistrySource(ctx, t, func(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
		assert.Equal(t, "example.com/download/name", downloadURL)

		// Create tar data and gzip it
		tarData := createTestTarData(t)
		var gzipBuf bytes.Buffer
		gw := gzip.NewWriter(&gzipBuf)
		_, err := gw.Write(tarData)
		require.NoError(t, err)
		require.NoError(t, gw.Close())

		return io.NopCloser(&gzipBuf), nil
	})

	testTemplateDownload(ctx, t, source)
}

//nolint:paralleltest // replaces global backend instance
func TestVCSBasedTemplateNames(t *testing.T) {
	ctx := testContext(t)
	mockRegistry := &backend.MockCloudRegistry{
		ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			assert.Nil(t, name)
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				if !yield(apitype.TemplateMetadata{
					Name:      "gh-org/repo/name",
					Source:    "github",
					Publisher: "pulumi-org",
				}, nil) {
					return
				}
				if !yield(apitype.TemplateMetadata{
					Name:      "gl-org/repo/name",
					Source:    "gitlab",
					Publisher: "pulumi-org",
				}, nil) {
					return
				}
				if !yield(apitype.TemplateMetadata{
					Name:      "just/has/slashes",
					Source:    "private",
					Publisher: "pulumi-org",
				}, nil) {
					return
				}
			}
		},
	}
	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry { return mockRegistry },
	}
	testutil.MockBackendInstance(t, mockBackend)
	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{ /* panic on use */ })

	source := newImpl(ctx, "", ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
			"PULUMI_EXPERIMENTAL":             "true",
		}))

	templates, err := source.Templates()
	require.NoError(t, err)
	require.Len(t, templates, 3)

	assert.Equal(t, "name", templates[0].Name())
	assert.Equal(t, "name", templates[1].Name())
	assert.Equal(t, "just/has/slashes", templates[2].Name())
}

//nolint:paralleltest // replaces global backend instance
func TestVCSBasedTemplateNameFilter(t *testing.T) {
	ctx := testContext(t)
	mockRegistry := &backend.MockCloudRegistry{
		ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			assert.Nil(t, name)
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				if !yield(apitype.TemplateMetadata{
					Name:        "gh-org/repo/target",
					Source:      "github",
					Description: ref("This is from GH"),
					Publisher:   "pulumi-org",
				}, nil) {
					return
				}
				if !yield(apitype.TemplateMetadata{
					Name:      "gl-org/repo/name",
					Source:    "gitlab",
					Publisher: "pulumi-org",
				}, nil) {
					return
				}
				if !yield(apitype.TemplateMetadata{
					Name:        "target",
					Source:      "private",
					Description: ref("This is from the registry"),
					Publisher:   "pulumi-org",
				}, nil) {
					return
				}
			}
		},
	}
	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry { return mockRegistry },
	}
	testutil.MockBackendInstance(t, mockBackend)
	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{ /* panic on use */ })

	source := newImpl(ctx, "target", ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
			"PULUMI_EXPERIMENTAL":             "true",
		}))

	templates, err := source.Templates()
	require.NoError(t, err)
	require.Len(t, templates, 2)

	assert.Equal(t, "target", templates[0].Name())
	assert.Equal(t, "This is from GH", templates[0].ProjectDescription())
	assert.Equal(t, "target", templates[1].Name())
	assert.Equal(t, "This is from the registry", templates[1].ProjectDescription())
}

func templateRepository(repo workspace.TemplateRepository, err error) getWorkspaceTemplateFunc {
	return func(ctx context.Context, templateNamePathOrURL string, offline bool,
		templateKind workspace.TemplateKind,
	) (workspace.TemplateRepository, error) {
		return repo, err
	}
}

func testContext(t *testing.T) context.Context {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	return ctx
}

func ref[T any](v T) *T { return &v }

//nolint:paralleltest // replaces global backend instance
func TestRegistryTemplateResolution(t *testing.T) {
	ctx := testContext(t)

	mockRegistry := &backend.MockCloudRegistry{
		ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				yield(apitype.TemplateMetadata{
					Name:        "csharp-documented",
					Source:      "private",
					Publisher:   "pulumi_local",
					Description: ref("A C# template"),
				}, nil)
				yield(apitype.TemplateMetadata{
					Name:      "csharp-documented",
					Source:    "github",
					Publisher: "different-org",
				}, nil)
				yield(apitype.TemplateMetadata{
					Name:      "gh-org/repo/target",
					Source:    "github",
					Publisher: "pulumi-org",
				}, nil)
				yield(apitype.TemplateMetadata{
					Name:        "whatever-template",
					Source:      "private",
					Publisher:   "test-org",
					Description: ref("A template with special chars"),
				}, nil)
			}
		},
	}
	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry { return mockRegistry },
	}
	testutil.MockBackendInstance(t, mockBackend)
	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{ /* panic on use */ })

	testCases := []struct {
		name                string
		templateURL         string
		shouldMatch         bool
		expectedName        string
		description         string
		expectSpecificError string
	}{
		{
			name:         "registry URL full format",
			templateURL:  "registry://templates/private/pulumi_local/csharp-documented",
			shouldMatch:  true,
			expectedName: "csharp-documented",
			description:  "A C# template",
		},
		{
			name:         "registry URL with version",
			templateURL:  "registry://templates/private/pulumi_local/csharp-documented@latest",
			shouldMatch:  true,
			expectedName: "csharp-documented",
			description:  "A C# template",
		},
		{
			name:         "partial URL format",
			templateURL:  "private/pulumi_local/csharp-documented",
			shouldMatch:  true,
			expectedName: "csharp-documented",
			description:  "A C# template",
		},
		{
			name:         "partial URL with version",
			templateURL:  "private/pulumi_local/csharp-documented@latest",
			shouldMatch:  true,
			expectedName: "csharp-documented",
			description:  "A C# template",
		},
		{
			name:         "VCS template display name matching",
			templateURL:  "target",
			shouldMatch:  true,
			expectedName: "target",
		},
		{
			name:                "wrong resource type does not match",
			templateURL:         "registry://packages/private/pulumi_local/csharp-documented",
			shouldMatch:         false,
			expectSpecificError: "resource type 'packages' is not valid for templates",
		},
		{
			name:         "parsing failure falls back to exact name match",
			templateURL:  "whatever-template",
			shouldMatch:  true,
			expectedName: "whatever-template",
			description:  "A template with special chars",
		},
		{
			name:        "nonexistent template returns not found",
			templateURL: "nonexistent/template/name",
			shouldMatch: false,
		},
		{
			name:                "malformed URL with no match",
			templateURL:         "registry://templates/a/b/c/d/e",
			shouldMatch:         false,
			expectSpecificError: expectedRegistryFormatError,
		},
		{
			name:        "git repo URL should not trigger registry errors",
			templateURL: "https://github.com/user/repo",
			shouldMatch: false,
		},
		{
			name:        "ssh git URL should not trigger registry errors",
			templateURL: "git@github.com:user/repo.git",
			shouldMatch: false,
		},
		{
			name:        "git repo URL with path should not trigger registry errors",
			templateURL: "https://github.com/user/repo/tree/main/templates/example",
			shouldMatch: false,
		},
		{
			name:                "wrong resource type - unknown resource type",
			templateURL:         "registry://unknown/private/publisher/name",
			shouldMatch:         false,
			expectSpecificError: "resource type 'unknown' is not valid for templates",
		},
		{
			name:                "malformed registry URL - missing parts",
			templateURL:         "registry://templates/private",
			shouldMatch:         false,
			expectSpecificError: expectedRegistryFormatError,
		},
		{
			name:        "malformed partial URL - too many parts",
			templateURL: "a/b/c/d/e",
			shouldMatch: false,
			// This should fall back to name matching (structural error), not show specific error
		},
		{
			name:                "malformed registry URL - empty version",
			templateURL:         "registry://templates/private/publisher/name@",
			shouldMatch:         false,
			expectSpecificError: "missing version",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			source := newImpl(ctx, tc.templateURL, ScopeAll, workspace.TemplateKindPulumiProject,
				templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
				env.NewEnv(env.MapStore{
					"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
					"PULUMI_EXPERIMENTAL":             "true",
				}))

			templates, err := source.Templates()
			if tc.shouldMatch {
				require.NoError(t, err)
				require.Len(t, templates, 1)
				assert.Equal(t, tc.expectedName, templates[0].Name())
				if tc.description != "" {
					assert.Equal(t, tc.description, templates[0].ProjectDescription())
				}
			} else {
				require.Error(t, err)
				if tc.expectSpecificError != "" {
					assert.Contains(t, err.Error(), tc.expectSpecificError)
				} else {
					var templateNotFound workspace.TemplateNotFoundError
					assert.ErrorAs(t, err, &templateNotFound)
				}
			}
		})
	}
}
