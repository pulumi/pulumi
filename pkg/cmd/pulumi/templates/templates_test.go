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
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // replaces global backend instance
func TestFilterOnName(t *testing.T) {
	ctx := testContext(t)

	template1 := &apitype.PulumiTemplateRemote{
		ProjectTemplate: apitype.ProjectTemplate{},
		Name:            "name1",
		SourceName:      "source1",
		TemplateURL:     "example.com/source1/name1",
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
	setBackend(t, mockBackend)

	setLoginManager(t, &cmdBackend.MockLoginManager{
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
		false, templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
	)

	template, err := source.Templates()
	require.NoError(t, err)
	assert.Equal(t,
		[]Template{orgTemplate{t: template1, org: "org1", source: source, backend: cmdBackend.BackendInstance}},
		template)
}

//nolint:paralleltest // replaces global backend instance
func TestMultipleTemplateSources(t *testing.T) {
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
	setBackend(t, mockBackend)

	setLoginManager(t, &cmdBackend.MockLoginManager{
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
		false, repoTemplates,
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
func TestSurfaceListTemplateErrors(t *testing.T) {
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
	setBackend(t, mockBackend)

	setLoginManager(t, &cmdBackend.MockLoginManager{
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
		false, templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
	)

	_, err := source.Templates()
	assert.ErrorIs(t, err, somethingWentWrong)
}

//nolint:paralleltest // replaces global backend instance
func TestSurfaceOnEmptyError(t *testing.T) {
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
	setBackend(t, mockBackend)

	setLoginManager(t, &cmdBackend.MockLoginManager{
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
		false, templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
	)

	_, err := source.Templates()
	var expected workspace.TemplateNotFoundError
	assert.ErrorAsf(t, err, &expected, "what's in %#v", source.errorOnEmpty)
}

//nolint:paralleltest // replaces global backend instance
func TestOrgTemplateDownload(t *testing.T) {
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
	setBackend(t, mockBackend)

	setLoginManager(t, &cmdBackend.MockLoginManager{
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
		false, templateRepository(workspace.TemplateRepository{}, workspace.TemplateNotFoundError{}),
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

func templateRepository(repo workspace.TemplateRepository, err error) getWorkspaceTemplateFunc {
	return func(ctx context.Context, templateNamePathOrURL string, offline bool,
		templateKind workspace.TemplateKind,
	) (workspace.TemplateRepository, error) {
		return repo, err
	}
}

func setBackend(t *testing.T, backend backend.Backend) {
	oldBackend := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = backend
	t.Cleanup(func() { cmdBackend.BackendInstance = oldBackend })
}

func setLoginManager(t *testing.T, lm cmdBackend.LoginManager) {
	oldLM := cmdBackend.DefaultLoginManager
	cmdBackend.DefaultLoginManager = lm
	t.Cleanup(func() { cmdBackend.DefaultLoginManager = oldLM })
}

func testContext(t *testing.T) context.Context {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	return ctx
}
