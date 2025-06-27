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
		assert.NoError(t, source.Close())
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

//nolint:paralleltest // replaces global backend instance
func TestTemplateDownload_Registry(t *testing.T) {
	ctx := testContext(t)

	pulumiYAML := `name: template1
runtime: dotnet
description: An ASP.NET application running a simple container in a EKS Cluster
`
	anotherFile := `This is another file`

	mockRegistry := &backend.MockCloudRegistry{
		ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				yield(apitype.TemplateMetadata{
					Name:        "name1",
					DownloadURL: "example.com/download/name",
				}, nil)
			}
		},
		DownloadTemplateF: func(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
			assert.Equal(t, "example.com/download/name", downloadURL)
			var target bytes.Buffer
			w := tar.NewWriter(&target)

			// Write Pulumi.yaml
			require.NoError(t, w.WriteHeader(&tar.Header{
				Name: "Pulumi.yaml",
				Size: int64(len(pulumiYAML)),
				Mode: 0o600,
			}))
			_, err := w.Write([]byte(pulumiYAML))
			require.NoError(t, err)

			// Write other.txt
			require.NoError(t, w.WriteHeader(&tar.Header{
				Name: "other.txt",
				Size: int64(len(anotherFile)),
				Mode: 0o600,
			}))
			_, err = w.Write([]byte(anotherFile))
			require.NoError(t, err)

			// Close the writers
			require.NoError(t, w.Close())

			return io.NopCloser(&target), nil
		},
	}
	mockBackend := &backend.MockBackend{
		GetReadOnlyCloudRegistryF: func() registry.Registry { return mockRegistry },
	}
	testutil.MockBackendInstance(t, mockBackend)
	testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{ /* panic on use */ })

	source := newImpl(ctx, "name1",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
		env.NewEnv(env.MapStore{
			"PULUMI_DISABLE_REGISTRY_RESOLVE": "false",
			"PULUMI_EXPERIMENTAL":             "true",
		}),
	)

	template, err := source.Templates()
	require.NoError(t, err)
	require.Len(t, template, 1)
	assert.Equal(t, "name1", template[0].Name())
	t.Cleanup(func() {
		assert.NoError(t, source.Close())
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
