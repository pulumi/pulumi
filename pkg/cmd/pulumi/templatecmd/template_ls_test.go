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

func ptr(s string) *string {
	return &s
}

func testTemplates() []apitype.TemplateMetadata {
	return []apitype.TemplateMetadata{
		{
			Name:        "template-one",
			Publisher:   "org-a",
			Source:      "private",
			DisplayName: "Template One",
			Description: ptr("First template"),
			Language:    "python",
			Visibility:  apitype.VisibilityPrivate,
			UpdatedAt:   time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			Name:        "template-two",
			Publisher:   "org-a",
			Source:      "private",
			DisplayName: "Template Two",
			Description: nil, // nil description
			Language:    "typescript",
			Visibility:  apitype.VisibilityPrivate,
			UpdatedAt:   time.Date(2025, 2, 20, 14, 30, 0, 0, time.UTC),
		},
		{
			Name:        "template-three",
			Publisher:   "org-b",
			Source:      "private",
			DisplayName: "Template Three",
			Description: ptr("Third template from different org"),
			Language:    "go",
			Visibility:  apitype.VisibilityPublic,
			UpdatedAt:   time.Date(2025, 3, 10, 8, 0, 0, 0, time.UTC),
		},
	}
}

func mockRegistryWithTemplates(templates []apitype.TemplateMetadata) *backend.MockBackend {
	return &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
						return func(yield func(apitype.TemplateMetadata, error) bool) {
							for _, t := range templates {
								if name != nil && t.Name != *name {
									continue
								}
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
func TestTemplateLsCmd_Success(t *testing.T) {
	templates := testTemplates()
	testutil.MockBackendInstance(t, mockRegistryWithTemplates(templates))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	// Check table headers
	require.Contains(t, output, "NAME")
	require.Contains(t, output, "PUBLISHER")
	require.Contains(t, output, "LANGUAGE")
	// Check template names (short names in table)
	require.Contains(t, output, "template-one")
	require.Contains(t, output, "template-two")
	require.Contains(t, output, "template-three")
	// Check publishers
	require.Contains(t, output, "org-a")
	require.Contains(t, output, "org-b")
	// Check languages
	require.Contains(t, output, "python")
	require.Contains(t, output, "typescript")
	require.Contains(t, output, "go")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_WithNameFilter(t *testing.T) {
	templates := testTemplates()
	testutil.MockBackendInstance(t, mockRegistryWithTemplates(templates))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--name", "template-one"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "template-one")
	require.NotContains(t, output, "template-two")
	require.NotContains(t, output, "template-three")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_ValidatesNameFilterPassedToAPI(t *testing.T) {
	var capturedFilter *string

	testutil.MockBackendInstance(t, &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
						capturedFilter = name
						return func(yield func(apitype.TemplateMetadata, error) bool) {
							// Empty iterator
						}
					},
				},
			}, nil
		},
	})

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--name", "specific-template"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)
	require.NotNil(t, capturedFilter)
	require.Equal(t, "specific-template", *capturedFilter)
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_CombinedFilters(t *testing.T) {
	templates := testTemplates() // Contains org-a and org-b templates
	testutil.MockBackendInstance(t, mockRegistryWithTemplates(templates))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--publisher", "org-a", "--name", "template-one"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "template-one")
	require.NotContains(t, output, "template-two")
	require.NotContains(t, output, "template-three")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_WithPublisherFilter(t *testing.T) {
	templates := testTemplates()
	testutil.MockBackendInstance(t, mockRegistryWithTemplates(templates))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--publisher", "org-a"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	// Check that org-a templates are shown (short names in table)
	require.Contains(t, output, "template-one")
	require.Contains(t, output, "template-two")
	// Check that org-b template is filtered out
	require.NotContains(t, output, "template-three")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_EmptyResult(t *testing.T) {
	testutil.MockBackendInstance(t, mockRegistryWithTemplates([]apitype.TemplateMetadata{}))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "No templates found")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_EmptyResultWithPublisher(t *testing.T) {
	testutil.MockBackendInstance(t, mockRegistryWithTemplates([]apitype.TemplateMetadata{}))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--publisher", "nonexistent-org"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "No templates found for publisher")
	require.Contains(t, output, "nonexistent-org")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_JSON(t *testing.T) {
	templates := testTemplates()
	testutil.MockBackendInstance(t, mockRegistryWithTemplates(templates))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--json"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var output []templateListJSON
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	require.Len(t, output, 3)

	// Verify first template
	require.Equal(t, "template-one", output[0].Name)
	require.Equal(t, "org-a", output[0].Publisher)
	require.Equal(t, "python", output[0].Language)
	require.Equal(t, "private", output[0].Visibility)
	require.NotNil(t, output[0].Description)
	require.Equal(t, "First template", *output[0].Description)

	// Verify second template has nil description
	require.Equal(t, "template-two", output[1].Name)
	require.Nil(t, output[1].Description)

	// Verify third template
	require.Equal(t, "template-three", output[2].Name)
	require.Equal(t, "org-b", output[2].Publisher)
	require.Equal(t, "public", output[2].Visibility)
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_JSONEmpty(t *testing.T) {
	testutil.MockBackendInstance(t, mockRegistryWithTemplates([]apitype.TemplateMetadata{}))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--json"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	output := buf.String()
	require.Equal(t, "[]\n", output)
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_RegistryNotSupported(t *testing.T) {
	testutil.MockBackendInstance(t, &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return nil, errors.New("registry not supported")
		},
	})

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "backend does not support Private Registry operations")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_IteratorError(t *testing.T) {
	testutil.MockBackendInstance(t, &backend.MockBackend{
		GetCloudRegistryF: func() (backend.CloudRegistry, error) {
			return &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
						return func(yield func(apitype.TemplateMetadata, error) bool) {
							// Return one template then an error
							yield(apitype.TemplateMetadata{
								Name:      "template-one",
								Publisher: "org-a",
							}, nil)
							yield(apitype.TemplateMetadata{}, errors.New("iterator failed"))
						}
					},
				},
			}, nil
		},
	})

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to list templates")
	require.Contains(t, err.Error(), "iterator failed")
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateLsCmd_NilFields(t *testing.T) {
	// Test that templates with nil fields don't crash
	templates := []apitype.TemplateMetadata{
		{
			Name:        "minimal-template",
			Publisher:   "org",
			Language:    "python",
			Visibility:  apitype.VisibilityPrivate,
			Description: nil,
			RepoSlug:    nil,
			Metadata:    nil,
			UpdatedAt:   time.Now(),
		},
	}

	testutil.MockBackendInstance(t, mockRegistryWithTemplates(templates))

	cmd := newTemplateLsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--json"})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var output []templateListJSON
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)
	require.Len(t, output, 1)
	require.Equal(t, "minimal-template", output[0].Name)
	require.Nil(t, output[0].Description)
}
