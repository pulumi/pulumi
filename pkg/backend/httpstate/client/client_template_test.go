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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTemplateArchiveData = []byte("fake-tar-gz-data")

type templateTestCase struct {
	name             string
	setupServer      func(blobStorage *httptest.Server) *httptest.Server
	setupBlobStorage func() *httptest.Server
	source           string
	publisher        string
	templateName     string
	version          semver.Version
	archiveData      []byte
	errorMessage     string
	httpClient       *http.Client
}

func TestStartTemplatePublish(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		source         string
		publisher      string
		templateName   string
		version        semver.Version
		expectedError  string
		validateResult func(t *testing.T, resp *StartTemplatePublishResponse)
	}{
		{
			name: "SuccessfulStartPublish",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/preview/registry/templates/private/test-publisher/test-template/versions" {
						w.WriteHeader(http.StatusAccepted)
						response := StartTemplatePublishResponse{
							OperationID: "test-operation-id",
							UploadURLs: TemplateUploadURLs{
								Archive: "https://example.com/upload/archive",
							},
						}
						require.NoError(t, json.NewEncoder(w).Encode(response))
					} else {
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			source:       "private",
			publisher:    "test-publisher",
			templateName: "test-template",
			version:      semver.MustParse("1.0.0"),
			validateResult: func(t *testing.T, resp *StartTemplatePublishResponse) {
				assert.Equal(t, TemplatePublishOperationID("test-operation-id"), resp.OperationID)
				assert.Equal(t, "https://example.com/upload/archive", resp.UploadURLs.Archive)
			},
		},
		{
			name: "FailedStartPublish",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, err := w.Write([]byte("Internal Server Error"))
					require.NoError(t, err)
				}))
			},
			source:        "private",
			publisher:     "test-publisher",
			templateName:  "test-template",
			version:       semver.MustParse("1.0.0"),
			expectedError: "start template publish failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := tt.setupServer()
			defer server.Close()

			client := &Client{
				apiURL:   server.URL,
				apiToken: "fake-token",
				restClient: &defaultRESTClient{
					client: &defaultHTTPClient{
						client: http.DefaultClient,
					},
				},
			}

			resp, err := client.StartTemplatePublish(context.Background(), tt.source, tt.publisher, tt.templateName, tt.version)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				if tt.validateResult != nil {
					tt.validateResult(t, resp)
				}
			}
		})
	}
}

func TestCompleteTemplatePublish(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		source        string
		publisher     string
		templateName  string
		version       semver.Version
		operationID   TemplatePublishOperationID
		expectedError string
	}{
		{
			name: "SuccessfulCompletePublish",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/preview/registry/templates/private/test-publisher/test-template/versions/1.0.0/complete" {
						w.WriteHeader(http.StatusCreated)
						response := PublishTemplateVersionCompleteResponse{}
						require.NoError(t, json.NewEncoder(w).Encode(response))
					} else {
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			source:       "private",
			publisher:    "test-publisher",
			templateName: "test-template",
			version:      semver.MustParse("1.0.0"),
			operationID:  "test-operation-id",
		},
		{
			name: "FailedCompletePublish",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, err := w.Write([]byte("Failed to complete"))
					require.NoError(t, err)
				}))
			},
			source:        "private",
			publisher:     "test-publisher",
			templateName:  "test-template",
			version:       semver.MustParse("1.0.0"),
			operationID:   "test-operation-id",
			expectedError: "failed to complete template publishing operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := tt.setupServer()
			defer server.Close()

			client := &Client{
				apiURL:   server.URL,
				apiToken: "fake-token",
				restClient: &defaultRESTClient{
					client: &defaultHTTPClient{
						client: http.DefaultClient,
					},
				},
			}

			err := client.CompleteTemplatePublish(
				context.Background(),
				tt.source,
				tt.publisher,
				tt.templateName,
				tt.version,
				tt.operationID,
			)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPublishTemplate_Integration(t *testing.T) {
	t.Parallel()

	tests := []templateTestCase{
		{
			name: "SuccessfulPublish",
			setupBlobStorage: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			setupServer: func(blobStorage *httptest.Server) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/preview/registry/templates/private/test-publisher/test-template/versions":
						w.WriteHeader(http.StatusAccepted)
						response := StartTemplatePublishResponse{
							OperationID: "test-operation-id",
							UploadURLs: TemplateUploadURLs{
								Archive: blobStorage.URL + "/upload/archive",
							},
						}
						require.NoError(t, json.NewEncoder(w).Encode(response))

					case "/api/preview/registry/templates/private/test-publisher/test-template/versions/1.0.0/complete":
						w.WriteHeader(http.StatusCreated)
					}
				}))
			},
			source:       "private",
			publisher:    "test-publisher",
			templateName: "test-template",
			version:      semver.MustParse("1.0.0"),
			archiveData:  testTemplateArchiveData,
		},
		{
			name: "FailedStartPublish",
			setupBlobStorage: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			setupServer: func(blobStorage *httptest.Server) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/preview/registry/templates/private/test-publisher/test-template/versions" {
						w.WriteHeader(http.StatusInternalServerError)
						_, err := w.Write([]byte("Internal Server Error"))
						require.NoError(t, err)
					}
				}))
			},
			source:       "private",
			publisher:    "test-publisher",
			templateName: "test-template",
			version:      semver.MustParse("1.0.0"),
			archiveData:  testTemplateArchiveData,
			errorMessage: "start template publish failed",
		},
		{
			name: "FailedArchiveUpload",
			setupBlobStorage: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/upload/archive" {
						w.WriteHeader(http.StatusForbidden)
					} else {
						w.WriteHeader(http.StatusOK)
					}
				}))
			},
			setupServer: func(blobStorage *httptest.Server) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/preview/registry/templates/private/test-publisher/test-template/versions":
						w.WriteHeader(http.StatusAccepted)
						response := StartTemplatePublishResponse{
							OperationID: "test-operation-id",
							UploadURLs: TemplateUploadURLs{
								Archive: blobStorage.URL + "/upload/archive",
							},
						}
						require.NoError(t, json.NewEncoder(w).Encode(response))
					}
				}))
			},
			source:       "private",
			publisher:    "test-publisher",
			templateName: "test-template",
			version:      semver.MustParse("1.0.0"),
			archiveData:  testTemplateArchiveData,
			errorMessage: "upload failed with status 403",
		},
		{
			name: "FailedCompletePublish",
			setupBlobStorage: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			setupServer: func(blobStorage *httptest.Server) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/preview/registry/templates/private/test-publisher/test-template/versions":
						w.WriteHeader(http.StatusAccepted)
						response := StartTemplatePublishResponse{
							OperationID: "test-operation-id",
							UploadURLs: TemplateUploadURLs{
								Archive: blobStorage.URL + "/upload/archive",
							},
						}
						require.NoError(t, json.NewEncoder(w).Encode(response))
					case "/api/preview/registry/templates/private/test-publisher/test-template/versions/1.0.0/complete":
						w.WriteHeader(http.StatusInternalServerError)
						_, err := w.Write([]byte("Failed to complete"))
						require.NoError(t, err)
					}
				}))
			},
			source:       "private",
			publisher:    "test-publisher",
			templateName: "test-template",
			version:      semver.MustParse("1.0.0"),
			archiveData:  testTemplateArchiveData,
			errorMessage: "failed to complete template publish",
		},
		{
			name: "NetworkFailure",
			httpClient: &http.Client{
				Transport: &errorTransport{
					roundTripFunc: func(req *http.Request) (*http.Response, error) {
						return nil, errors.New("simulated network error")
					},
				},
			},
			setupBlobStorage: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			setupServer: func(blobStorage *httptest.Server) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/preview/registry/templates/private/test-publisher/test-template/versions":
						w.WriteHeader(http.StatusAccepted)
						response := StartTemplatePublishResponse{
							OperationID: "test-operation-id",
							UploadURLs: TemplateUploadURLs{
								Archive: blobStorage.URL + "/upload/archive",
							},
						}
						require.NoError(t, json.NewEncoder(w).Encode(response))
					}
				}))
			},
			source:       "private",
			publisher:    "test-publisher",
			templateName: "test-template",
			version:      semver.MustParse("1.0.0"),
			archiveData:  testTemplateArchiveData,
			errorMessage: "simulated network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			blobStorage := tt.setupBlobStorage()
			defer blobStorage.Close()
			server := tt.setupServer(blobStorage)
			defer server.Close()

			var httpClient *http.Client
			if tt.httpClient != nil {
				httpClient = tt.httpClient
			} else {
				httpClient = http.DefaultClient
			}

			// Create a mock cloud registry that uses the HTTP client methods
			client := &Client{
				apiURL:     server.URL,
				apiToken:   "fake-token",
				httpClient: httpClient,
				restClient: &defaultRESTClient{
					client: &defaultHTTPClient{
						client: httpClient,
					},
				},
			}

			// Test the full template publish workflow through the cloud registry
			registry := newCloudRegistry(client)

			op := templatePublishOp{
				Source:    tt.source,
				Publisher: tt.publisher,
				Name:      tt.templateName,
				Version:   tt.version,
				Archive:   bytes.NewReader(tt.archiveData),
			}

			err := registry.PublishTemplate(context.Background(), op)

			if tt.errorMessage != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper type to match the backend.TemplatePublishOp interface for testing
type templatePublishOp struct {
	Source    string
	Publisher string
	Name      string
	Version   semver.Version
	Archive   *bytes.Reader
}

// Mock implementation of newCloudRegistry for testing
func newCloudRegistry(client *Client) *testCloudRegistry {
	return &testCloudRegistry{cl: client}
}

type testCloudRegistry struct {
	cl *Client
}

func (r *testCloudRegistry) PublishTemplate(ctx context.Context, op templatePublishOp) error {
	startResp, err := r.cl.StartTemplatePublish(ctx, op.Source, op.Publisher, op.Name, op.Version)
	if err != nil {
		return err
	}

	uploadURL := startResp.UploadURLs.Archive
	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, op.Archive)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/gzip")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return errors.New("upload failed with status " + resp.Status)
	}

	err = r.cl.CompleteTemplatePublish(ctx, op.Source, op.Publisher, op.Name, op.Version, startResp.OperationID)
	if err != nil {
		return err
	}

	return nil
}

func TestListTemplates(t *testing.T) {
	t.Parallel()

	t.Run("no-continuation-token", func(t *testing.T) {
		t.Parallel()

		// Create a mock response with template metadata
		desc1 := "First template"
		desc2 := "Second template"
		expectedTemplates := []apitype.TemplateMetadata{
			{
				Name:        "my-template-1",
				Publisher:   "my-publisher",
				Source:      "my-source",
				Description: &desc1,
				Language:    "go",
				Visibility:  apitype.VisibilityPrivate,
			},
			{
				Name:        "my-template-2",
				Publisher:   "my-publisher",
				Source:      "my-source",
				Description: &desc2,
				Language:    "typescript",
				Visibility:  apitype.VisibilityPrivate,
			},
		}

		mockResponse := apitype.ListTemplatesResponse{
			Templates: expectedTemplates,
		}

		// Set up mock server
		mockServer := newMockServerRequestProcessor(200, func(req *http.Request) string {
			assert.Contains(t, req.URL.String(), "/api/preview/registry/templates?limit=499")
			assert.Equal(t, "GET", req.Method)

			data, err := json.Marshal(mockResponse)
			require.NoError(t, err)
			return string(data)
		})
		defer mockServer.Close()

		mockClient := newMockClient(mockServer)

		// Call ListTemplates and collect results
		searchName := "my-template"
		searchResults := []apitype.TemplateMetadata{}
		for tmpl, err := range mockClient.ListTemplates(context.Background(), &searchName) {
			require.NoError(t, err)
			searchResults = append(searchResults, tmpl)
		}
		assert.Equal(t, expectedTemplates, searchResults)
	})

	t.Run("with-continuation-token", func(t *testing.T) {
		t.Parallel()

		// First page response
		desc1 := "First template"
		desc2 := "Second template"
		desc3 := "Third template"
		firstPageTemplates := []apitype.TemplateMetadata{
			{
				Name:        "my-template-1",
				Publisher:   "my-publisher",
				Source:      "my-source",
				Description: &desc1,
				Language:    "go",
				Visibility:  apitype.VisibilityPrivate,
			},
		}

		secondPageTemplates := []apitype.TemplateMetadata{
			{
				Name:        "my-template-2",
				Publisher:   "my-publisher",
				Source:      "my-source",
				Description: &desc2,
				Language:    "typescript",
				Visibility:  apitype.VisibilityPrivate,
			},
		}

		thirdPageTemplates := []apitype.TemplateMetadata{
			{
				Name:        "my-template-3",
				Publisher:   "my-publisher",
				Source:      "my-source",
				Description: &desc3,
				Language:    "python",
				Visibility:  apitype.VisibilityPrivate,
			},
		}

		// Track which request is being made
		requestCount := 0

		// Set up mock server
		mockServer := newMockServerRequestProcessor(200, func(req *http.Request) string {
			assert.Equal(t, "GET", req.Method)

			var responseData []byte
			var err error

			switch requestCount {
			case 0:
				assert.Equal(t, "/api/preview/registry/templates?limit=499&name=my-template", req.URL.String())
				assert.NotContains(t, "continuationToken", req.URL.String())

				responseData, err = json.Marshal(apitype.ListTemplatesResponse{
					Templates:         firstPageTemplates,
					ContinuationToken: ptr("next-page-token-1"),
				})
				require.NoError(t, err)
			case 1:
				assert.Equal(t,
					"/api/preview/registry/templates?limit=499&name=my-template&continuationToken=next-page-token-1",
					req.URL.String())

				responseData, err = json.Marshal(apitype.ListTemplatesResponse{
					Templates:         secondPageTemplates,
					ContinuationToken: ptr("next-page-token-2"),
				})
				require.NoError(t, err)
			case 2:
				assert.Equal(t,
					"/api/preview/registry/templates?limit=499&name=my-template&continuationToken=next-page-token-2",
					req.URL.String())

				responseData, err = json.Marshal(apitype.ListTemplatesResponse{
					Templates: thirdPageTemplates,
				})
				require.NoError(t, err)
			}

			requestCount++
			return string(responseData)
		})
		defer mockServer.Close()

		mockClient := newMockClient(mockServer)

		searchName := "my-template"
		searchResults := []apitype.TemplateMetadata{}
		for tmpl, err := range mockClient.ListTemplates(context.Background(), &searchName) {
			require.NoError(t, err)
			searchResults = append(searchResults, tmpl)
		}

		expectedTemplates := append(append(firstPageTemplates, secondPageTemplates...), thirdPageTemplates...)
		assert.Equal(t, expectedTemplates, searchResults)
		assert.Equal(t, 3, requestCount) // Ensure all three requests were made
	})
}
