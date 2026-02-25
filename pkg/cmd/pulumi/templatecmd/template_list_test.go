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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateListCmd_Run(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name        string
		templates   []apitype.TemplateMetadata
		listErr     error
		jsonOut     bool
		expectedErr string
		validate    func(t *testing.T, output string)
	}{
		{
			name: "list templates in console format",
			templates: []apitype.TemplateMetadata{
				{
					Name:       "my-template",
					Publisher:  "myorg",
					Source:     "private",
					Language:   "python",
					Visibility: apitype.VisibilityPrivate,
					UpdatedAt:  now,
				},
				{
					Name:       "another-template",
					Publisher:  "myorg",
					Source:     "private",
					Language:   "typescript",
					Visibility: apitype.VisibilityPrivate,
					UpdatedAt:  now,
				},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "my-template")
				assert.Contains(t, output, "another-template")
				assert.Contains(t, output, "myorg")
				assert.Contains(t, output, "python")
				assert.Contains(t, output, "typescript")
				assert.Contains(t, output, "NAME")
				assert.Contains(t, output, "PUBLISHER")
			},
		},
		{
			name: "list templates in JSON format",
			templates: []apitype.TemplateMetadata{
				{
					Name:       "test-template",
					Publisher:  "testorg",
					Source:     "private",
					Language:   "go",
					Visibility: apitype.VisibilityPrivate,
					UpdatedAt:  now,
				},
			},
			jsonOut: true,
			validate: func(t *testing.T, output string) {
				var result []templateSummaryJSON
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)
				require.Len(t, result, 1)
				assert.Equal(t, "test-template", result[0].Name)
				assert.Equal(t, "testorg", result[0].Publisher)
				assert.Equal(t, "go", result[0].Language)
				assert.Equal(t, "private", result[0].Visibility)
			},
		},
		{
			name:      "empty template list",
			templates: nil,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "No templates found.")
			},
		},
		{
			name:        "error listing templates",
			listErr:     errors.New("api error"),
			expectedErr: "failed to list templates",
		},
		{
			name: "templates sorted by publisher then name",
			templates: []apitype.TemplateMetadata{
				{
					Name:       "z-template",
					Publisher:  "a-org",
					Source:     "private",
					Visibility: apitype.VisibilityPrivate,
					UpdatedAt:  now,
				},
				{
					Name:       "a-template",
					Publisher:  "a-org",
					Source:     "private",
					Visibility: apitype.VisibilityPrivate,
					UpdatedAt:  now,
				},
				{
					Name:       "m-template",
					Publisher:  "b-org",
					Source:     "private",
					Visibility: apitype.VisibilityPrivate,
					UpdatedAt:  now,
				},
			},
			jsonOut: true,
			validate: func(t *testing.T, output string) {
				var result []templateSummaryJSON
				err := json.Unmarshal([]byte(output), &result)
				require.NoError(t, err)
				require.Len(t, result, 3)
				assert.Equal(t, "a-template", result[0].Name)
				assert.Equal(t, "a-org", result[0].Publisher)
				assert.Equal(t, "z-template", result[1].Name)
				assert.Equal(t, "a-org", result[1].Publisher)
				assert.Equal(t, "m-template", result[2].Name)
				assert.Equal(t, "b-org", result[2].Publisher)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCloudRegistry := &backend.MockCloudRegistry{
				Mock: registry.Mock{
					ListTemplatesF: func(
						_ context.Context, _ *string,
					) iter.Seq2[apitype.TemplateMetadata, error] {
						return func(yield func(apitype.TemplateMetadata, error) bool) {
							if tt.listErr != nil {
								yield(apitype.TemplateMetadata{}, tt.listErr)
								return
							}
							for _, tmpl := range tt.templates {
								if !yield(tmpl, nil) {
									return
								}
							}
						}
					},
				},
			}

			testutil.MockBackendInstance(t, &backend.MockBackend{
				GetCloudRegistryF: func() (backend.CloudRegistry, error) {
					return mockCloudRegistry, nil
				},
			})

			var buf bytes.Buffer
			cmd := &templateListCmd{stdout: &buf}
			err := cmd.Run(context.Background(), templateListArgs{jsonOut: tt.jsonOut})

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, buf.String())
				}
			}
		})
	}
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplateListCmd_BackendErrors(t *testing.T) {
	tests := []struct {
		name           string
		setupBackend   func(t *testing.T)
		expectedErrStr string
	}{
		{
			name: "error getting cloud registry",
			setupBackend: func(t *testing.T) {
				testutil.MockBackendInstance(t, &backend.MockBackend{
					GetCloudRegistryF: func() (backend.CloudRegistry, error) {
						return nil, errors.New("not supported")
					},
				})
			},
			expectedErrStr: "backend does not support Private Registry operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupBackend(t)

			var buf bytes.Buffer
			cmd := &templateListCmd{stdout: &buf}
			err := cmd.Run(context.Background(), templateListArgs{})

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrStr)
		})
	}
}
