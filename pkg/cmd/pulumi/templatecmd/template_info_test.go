// Copyright 2025, Pulumi Corporation.
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

package templatecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/stretchr/testify/require"
)

func testInfoTemplate() apitype.TemplateMetadata {
	return apitype.TemplateMetadata{
		Name:        "pulumi/my-org-templates/my-template",
		Publisher:   "my-org",
		Source:      "github",
		DisplayName: "My Template",
		Description: ptr("A sample template for testing"),
		Language:    "python",
		Visibility:  apitype.VisibilityPrivate,
		RepoSlug:    ptr("pulumi/my-org-templates"),
		UpdatedAt:   time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Metadata: map[string]string{
			"key": "value",
		},
	}
}

func mockRegistryWithListTemplatesForInfo(templates []apitype.TemplateMetadata) *backend.MockBackend {
	return &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
						return func(yield func(apitype.TemplateMetadata, error) bool) {
							for _, t := range templates {
								if !yield(t, nil) {
									return
								}
							}
						}
					},
				},
			}, nil
		},
	}
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_Success(t *testing.T) {
	tmpl := testInfoTemplate()
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{tmpl}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"pulumi/my-org-templates/my-template"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "Name: pulumi/my-org-templates/my-template")
	require.Contains(t, output, "Publisher: my-org")
	require.Contains(t, output, "Display Name: My Template")
	require.Contains(t, output, "Language: python")
	require.Contains(t, output, "Description: A sample template for testing")
	require.Contains(t, output, "Repository: pulumi/my-org-templates")
	require.Contains(t, output, "Updated: 2025-01-15T10:00:00Z")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_MatchBySuffix(t *testing.T) {
	tmpl := testInfoTemplate()
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{tmpl}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Just pass the last part of the name
	cmd.SetArgs([]string{"my-template"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "Name: pulumi/my-org-templates/my-template")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_MatchByPublisherAndName(t *testing.T) {
	tmpl := testInfoTemplate()
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{tmpl}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Use publisher/name format from ls output
	cmd.SetArgs([]string{"my-org/pulumi/my-org-templates/my-template"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "Name: pulumi/my-org-templates/my-template")
	require.Contains(t, output, "Publisher: my-org")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_AmbiguousMatch(t *testing.T) {
	templates := []apitype.TemplateMetadata{
		{
			Name:       "pulumi/org-a/my-template",
			Publisher:  "org-a",
			Source:     "github",
			Language:   "python",
			Visibility: apitype.VisibilityPrivate,
			UpdatedAt:  time.Now(),
		},
		{
			Name:       "pulumi/org-b/my-template",
			Publisher:  "org-b",
			Source:     "github",
			Language:   "typescript",
			Visibility: apitype.VisibilityPrivate,
			UpdatedAt:  time.Now(),
		},
	}
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo(templates))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"my-template"})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "ambiguous")
	require.Contains(t, err.Error(), "org-a/pulumi/org-a/my-template")
	require.Contains(t, err.Error(), "org-b/pulumi/org-b/my-template")
	require.Contains(t, err.Error(), "--publisher")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_DisambiguateWithPublisher(t *testing.T) {
	templates := []apitype.TemplateMetadata{
		{
			Name:       "pulumi/org-a/my-template",
			Publisher:  "org-a",
			Source:     "github",
			Language:   "python",
			Visibility: apitype.VisibilityPrivate,
			UpdatedAt:  time.Now(),
		},
		{
			Name:       "pulumi/org-b/my-template",
			Publisher:  "org-b",
			Source:     "github",
			Language:   "typescript",
			Visibility: apitype.VisibilityPrivate,
			UpdatedAt:  time.Now(),
		},
	}
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo(templates))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"my-template", "--publisher", "org-a"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "Publisher: org-a")
	require.Contains(t, output, "Language: python")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_JSON(t *testing.T) {
	tmpl := testInfoTemplate()
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{tmpl}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"my-template", "--json"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var output templateInfoJSON
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	require.Equal(t, "pulumi/my-org-templates/my-template", output.Name)
	require.Equal(t, "my-org", output.Publisher)
	require.Equal(t, "github", output.Source)
	require.Equal(t, "My Template", output.DisplayName)
	require.Equal(t, "python", output.Language)
	require.Equal(t, "private", output.Visibility)
	require.NotNil(t, output.Description)
	require.Equal(t, "A sample template for testing", *output.Description)
	require.NotNil(t, output.RepoSlug)
	require.Equal(t, "pulumi/my-org-templates", *output.RepoSlug)
	require.Equal(t, "2025-01-15T10:00:00Z", output.UpdatedAt)
	require.NotNil(t, output.Metadata)
	require.Equal(t, "value", output.Metadata["key"])
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_NotFound(t *testing.T) {
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"nonexistent-template"})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_NotFoundWithPublisher(t *testing.T) {
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"nonexistent-template", "--publisher", "my-org"})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
	require.Contains(t, err.Error(), "my-org")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_RegistryNotSupported(t *testing.T) {
	testutil.MockBackendInstance(t, &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return nil, errors.New("registry not supported")
		},
	})

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"my-template"})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "backend does not support Private Registry operations")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_MissingArgument(t *testing.T) {
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_NilOptionalFields(t *testing.T) {
	tmpl := apitype.TemplateMetadata{
		Name:        "pulumi/org/minimal-template",
		Publisher:   "org",
		Source:      "github",
		Language:    "python",
		Visibility:  apitype.VisibilityPrivate,
		Description: nil,
		RepoSlug:    nil,
		DisplayName: "",
		Metadata:    nil,
		UpdatedAt:   time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	testutil.MockBackendInstance(t, mockRegistryWithListTemplatesForInfo([]apitype.TemplateMetadata{tmpl}))

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"minimal-template"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "Name: pulumi/org/minimal-template")
	require.Contains(t, output, "Publisher: org")
	require.NotContains(t, output, "Display Name:")
	require.NotContains(t, output, "Description:")
	require.NotContains(t, output, "Repository:")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateInfoCmd_IteratorError(t *testing.T) {
	testutil.MockBackendInstance(t, &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
						return func(yield func(apitype.TemplateMetadata, error) bool) {
							yield(apitype.TemplateMetadata{}, errors.New("iterator failed"))
						}
					},
				},
			}, nil
		},
	})

	cmd := newTemplateInfoCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"my-template"})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to list templates")
	require.Contains(t, err.Error(), "iterator failed")
}
