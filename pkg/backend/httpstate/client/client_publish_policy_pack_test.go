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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishPolicyPack_AllAnalyzerInfoFieldsAreSent(t *testing.T) {
	t.Parallel()

	// Create comprehensive AnalyzerInfo with all possible fields
	analyzerInfo := plugin.AnalyzerInfo{
		Name:        "test-policy-pack",
		DisplayName: "Test Policy Pack",
		Version:     "1.2.3",
		Description: "A comprehensive test policy pack",
		Readme:      "# Test Policy Pack\n\nThis is a test policy pack for validation.",
		Provider:    "aws",
		Tags:        []string{"security", "compliance", "test"},
		Repository:  "https://github.com/example/test-policy-pack",
		Policies: []plugin.AnalyzerPolicyInfo{
			{
				Name:             "required-tags",
				DisplayName:      "Required Tags",
				Description:      "Ensures all resources have required tags",
				EnforcementLevel: apitype.Mandatory,
				Message:          "Resources must have required tags",
				Severity:         apitype.PolicySeverityHigh,
				Framework: &plugin.AnalyzerPolicyComplianceFramework{
					Name:          "SOC2",
					Version:       "2017",
					Reference:     "CC6.1",
					Specification: "System Operations - Logical Access",
				},
				Tags:             []string{"tagging", "governance"},
				RemediationSteps: "Add the required tags to the resource",
				URL:              "https://example.com/policies/required-tags",
				ConfigSchema: &plugin.AnalyzerPolicyConfigSchema{
					Properties: map[string]plugin.JSONSchema{
						"requiredTags": {
							"type":        "array",
							"description": "List of required tag keys",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
					Required: []string{"requiredTags"},
				},
			},
		},
	}

	var capturedRequest *apitype.CreatePolicyPackRequest

	// Create a test server to capture the request
	server := newMockServerRequestProcessor(200, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/policypacks") && r.Method == "POST" {
			// Capture the request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			capturedRequest = &apitype.CreatePolicyPackRequest{}
			err = json.Unmarshal(body, capturedRequest)
			require.NoError(t, err)

			// Return a successful response
			resp := apitype.CreatePolicyPackResponse{
				Version:   1,
				UploadURI: "http://" + r.Host + "/upload",
				RequiredHeaders: map[string]string{
					"Content-Type": "application/gzip",
				},
			}
			respJSON, err := json.Marshal(resp)
			require.NoError(t, err)
			return string(respJSON)
		}

		if strings.HasSuffix(r.URL.Path, "/upload") && r.Method == "PUT" {
			// Mock successful upload
			return ""
		}

		if strings.Contains(r.URL.Path, "/complete") && r.Method == "POST" {
			// Mock successful completion
			return ""
		}

		return ""
	})
	defer server.Close()

	// Create client
	client := newMockClient(server)

	// Create a mock archive
	archive := bytes.NewReader([]byte("mock-archive-data"))

	// Call PublishPolicyPack
	version, err := client.PublishPolicyPack(context.Background(), "test-org", analyzerInfo, archive)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version)

	// Verify all fields were sent correctly
	require.NotNil(t, capturedRequest, "Request should have been captured")

	// Verify top-level AnalyzerInfo fields
	assert.Equal(t, analyzerInfo.Name, capturedRequest.Name)
	assert.Equal(t, analyzerInfo.DisplayName, capturedRequest.DisplayName)
	assert.Equal(t, analyzerInfo.Version, capturedRequest.VersionTag)
	assert.Equal(t, analyzerInfo.Description, capturedRequest.Description)
	assert.Equal(t, analyzerInfo.Readme, capturedRequest.Readme)
	assert.Equal(t, analyzerInfo.Provider, capturedRequest.Provider)
	assert.Equal(t, analyzerInfo.Tags, capturedRequest.Tags)
	assert.Equal(t, analyzerInfo.Repository, capturedRequest.Repository)

	// Verify policies were converted correctly
	require.Len(t, capturedRequest.Policies, 1)
	policy := capturedRequest.Policies[0]
	expectedPolicy := analyzerInfo.Policies[0]

	assert.Equal(t, expectedPolicy.Name, policy.Name)
	assert.Equal(t, expectedPolicy.DisplayName, policy.DisplayName)
	assert.Equal(t, expectedPolicy.Description, policy.Description)
	assert.Equal(t, expectedPolicy.EnforcementLevel, policy.EnforcementLevel)
	assert.Equal(t, expectedPolicy.Message, policy.Message)
	assert.Equal(t, expectedPolicy.Severity, policy.Severity)
	assert.Equal(t, expectedPolicy.Tags, policy.Tags)
	assert.Equal(t, expectedPolicy.RemediationSteps, policy.RemediationSteps)
	assert.Equal(t, expectedPolicy.URL, policy.URL)

	// Verify compliance framework conversion
	require.NotNil(t, policy.Framework)
	assert.Equal(t, expectedPolicy.Framework.Name, policy.Framework.Name)
	assert.Equal(t, expectedPolicy.Framework.Version, policy.Framework.Version)
	assert.Equal(t, expectedPolicy.Framework.Reference, policy.Framework.Reference)
	assert.Equal(t, expectedPolicy.Framework.Specification, policy.Framework.Specification)

	// Verify config schema conversion
	require.NotNil(t, policy.ConfigSchema)
	assert.Equal(t, apitype.Object, policy.ConfigSchema.Type)
	assert.Equal(t, expectedPolicy.ConfigSchema.Required, policy.ConfigSchema.Required)
	require.Contains(t, policy.ConfigSchema.Properties, "requiredTags")
}

func TestPublishPolicyPack_EmptyOptionalFields(t *testing.T) {
	t.Parallel()

	// Create minimal AnalyzerInfo with only required fields
	analyzerInfo := plugin.AnalyzerInfo{
		Name:    "minimal-policy-pack",
		Version: "1.0.0",
		Policies: []plugin.AnalyzerPolicyInfo{
			{
				Name:             "basic-policy",
				EnforcementLevel: apitype.Advisory,
			},
		},
	}

	var capturedRequest *apitype.CreatePolicyPackRequest

	server := newMockServerRequestProcessor(200, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/policypacks") && r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			capturedRequest = &apitype.CreatePolicyPackRequest{}
			err = json.Unmarshal(body, capturedRequest)
			require.NoError(t, err)

			resp := apitype.CreatePolicyPackResponse{
				Version:   1,
				UploadURI: "http://" + r.Host + "/upload",
			}
			respJSON, err := json.Marshal(resp)
			require.NoError(t, err)
			return string(respJSON)
		}

		if strings.HasSuffix(r.URL.Path, "/upload") && r.Method == "PUT" {
			return ""
		}

		if strings.Contains(r.URL.Path, "/complete") && r.Method == "POST" {
			return ""
		}

		return ""
	})
	defer server.Close()

	client := newMockClient(server)
	archive := bytes.NewReader([]byte("mock-archive-data"))

	_, err := client.PublishPolicyPack(context.Background(), "test-org", analyzerInfo, archive)
	require.NoError(t, err)

	// Verify required fields are present
	require.NotNil(t, capturedRequest)
	assert.Equal(t, analyzerInfo.Name, capturedRequest.Name)
	assert.Equal(t, analyzerInfo.Version, capturedRequest.VersionTag)

	// Verify optional fields are properly handled (empty strings/nil should be omitted)
	assert.Empty(t, capturedRequest.DisplayName)
	assert.Empty(t, capturedRequest.Description)
	assert.Empty(t, capturedRequest.Readme)
	assert.Empty(t, capturedRequest.Provider)
	assert.Empty(t, capturedRequest.Tags)
	assert.Empty(t, capturedRequest.Repository)
}

func TestPublishPolicyPack_LegacyVersionHandling(t *testing.T) {
	t.Parallel()

	// Create AnalyzerInfo without version (legacy scenario)
	analyzerInfo := plugin.AnalyzerInfo{
		Name:    "legacy-policy-pack",
		Version: "", // Empty version to simulate legacy policy pack
		Policies: []plugin.AnalyzerPolicyInfo{
			{
				Name:             "legacy-policy",
				EnforcementLevel: apitype.Advisory,
			},
		},
	}

	var capturedRequest *apitype.CreatePolicyPackRequest

	server := newMockServerRequestProcessor(200, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/policypacks") && r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			capturedRequest = &apitype.CreatePolicyPackRequest{}
			err = json.Unmarshal(body, capturedRequest)
			require.NoError(t, err)

			resp := apitype.CreatePolicyPackResponse{
				Version:   42, // Server-assigned version
				UploadURI: "http://" + r.Host + "/upload",
			}
			respJSON, err := json.Marshal(resp)
			require.NoError(t, err)
			return string(respJSON)
		}

		if strings.HasSuffix(r.URL.Path, "/upload") && r.Method == "PUT" {
			return ""
		}

		if strings.Contains(r.URL.Path, "/complete") && r.Method == "POST" {
			return ""
		}

		return ""
	})
	defer server.Close()

	client := newMockClient(server)
	archive := bytes.NewReader([]byte("mock-archive-data"))

	version, err := client.PublishPolicyPack(context.Background(), "test-org", analyzerInfo, archive)
	require.NoError(t, err)

	// Verify that server-assigned version is returned when client version is empty
	assert.Equal(t, "42", version)

	// Verify empty version tag is sent
	require.NotNil(t, capturedRequest)
	assert.Empty(t, capturedRequest.VersionTag)
}

func TestPublishPolicyPack_PolicyConfigSchemaConversion(t *testing.T) {
	t.Parallel()

	// Create AnalyzerInfo with complex config schema
	analyzerInfo := plugin.AnalyzerInfo{
		Name:    "config-schema-test",
		Version: "1.0.0",
		Policies: []plugin.AnalyzerPolicyInfo{
			{
				Name:             "config-test-policy",
				EnforcementLevel: apitype.Mandatory,
				ConfigSchema: &plugin.AnalyzerPolicyConfigSchema{
					Properties: map[string]plugin.JSONSchema{
						"stringProp": {
							"type":        "string",
							"description": "A string property",
							"default":     "default-value",
						},
						"numberProp": {
							"type":    "number",
							"minimum": 0,
							"maximum": 100,
						},
						"booleanProp": {
							"type": "boolean",
						},
						"arrayProp": {
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"minItems": 1,
						},
						"objectProp": {
							"type": "object",
							"properties": map[string]interface{}{
								"nestedProp": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
					Required: []string{"stringProp", "numberProp"},
				},
			},
		},
	}

	var capturedRequest *apitype.CreatePolicyPackRequest

	server := newMockServerRequestProcessor(200, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/policypacks") && r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			capturedRequest = &apitype.CreatePolicyPackRequest{}
			err = json.Unmarshal(body, capturedRequest)
			require.NoError(t, err)

			resp := apitype.CreatePolicyPackResponse{
				Version:   1,
				UploadURI: "http://" + r.Host + "/upload",
			}
			respJSON, err := json.Marshal(resp)
			require.NoError(t, err)
			return string(respJSON)
		}

		if strings.HasSuffix(r.URL.Path, "/upload") && r.Method == "PUT" {
			return ""
		}

		if strings.Contains(r.URL.Path, "/complete") && r.Method == "POST" {
			return ""
		}

		return ""
	})
	defer server.Close()

	client := newMockClient(server)
	archive := bytes.NewReader([]byte("mock-archive-data"))

	_, err := client.PublishPolicyPack(context.Background(), "test-org", analyzerInfo, archive)
	require.NoError(t, err)

	// Verify config schema conversion
	require.NotNil(t, capturedRequest)
	require.Len(t, capturedRequest.Policies, 1)
	policy := capturedRequest.Policies[0]
	require.NotNil(t, policy.ConfigSchema)

	assert.Equal(t, apitype.Object, policy.ConfigSchema.Type)
	assert.Equal(t, []string{"stringProp", "numberProp"}, policy.ConfigSchema.Required)

	// Verify properties were marshaled correctly
	require.Contains(t, policy.ConfigSchema.Properties, "stringProp")
	require.Contains(t, policy.ConfigSchema.Properties, "numberProp")
	require.Contains(t, policy.ConfigSchema.Properties, "booleanProp")
	require.Contains(t, policy.ConfigSchema.Properties, "arrayProp")
	require.Contains(t, policy.ConfigSchema.Properties, "objectProp")

	// Verify we can unmarshal the properties back to check they were preserved
	var stringProp map[string]interface{}
	err = json.Unmarshal(*policy.ConfigSchema.Properties["stringProp"], &stringProp)
	require.NoError(t, err)
	assert.Equal(t, "string", stringProp["type"])
	assert.Equal(t, "A string property", stringProp["description"])
	assert.Equal(t, "default-value", stringProp["default"])
}

func TestPublishPolicyPack_NilConfigSchema(t *testing.T) {
	t.Parallel()

	// Create AnalyzerInfo with nil config schema
	analyzerInfo := plugin.AnalyzerInfo{
		Name:    "nil-config-test",
		Version: "1.0.0",
		Policies: []plugin.AnalyzerPolicyInfo{
			{
				Name:             "no-config-policy",
				EnforcementLevel: apitype.Advisory,
				ConfigSchema:     nil, // Explicitly nil
			},
		},
	}

	var capturedRequest *apitype.CreatePolicyPackRequest

	server := newMockServerRequestProcessor(200, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/policypacks") && r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			capturedRequest = &apitype.CreatePolicyPackRequest{}
			err = json.Unmarshal(body, capturedRequest)
			require.NoError(t, err)

			resp := apitype.CreatePolicyPackResponse{
				Version:   1,
				UploadURI: "http://" + r.Host + "/upload",
			}
			respJSON, err := json.Marshal(resp)
			require.NoError(t, err)
			return string(respJSON)
		}

		if strings.HasSuffix(r.URL.Path, "/upload") && r.Method == "PUT" {
			return ""
		}

		if strings.Contains(r.URL.Path, "/complete") && r.Method == "POST" {
			return ""
		}

		return ""
	})
	defer server.Close()

	client := newMockClient(server)
	archive := bytes.NewReader([]byte("mock-archive-data"))

	_, err := client.PublishPolicyPack(context.Background(), "test-org", analyzerInfo, archive)
	require.NoError(t, err)

	// Verify nil config schema is handled correctly
	require.NotNil(t, capturedRequest)
	require.Len(t, capturedRequest.Policies, 1)
	policy := capturedRequest.Policies[0]
	assert.Nil(t, policy.ConfigSchema)
}

func TestPublishPolicyPack_NilComplianceFramework(t *testing.T) {
	t.Parallel()

	// Create AnalyzerInfo with nil compliance framework
	analyzerInfo := plugin.AnalyzerInfo{
		Name:    "nil-framework-test",
		Version: "1.0.0",
		Policies: []plugin.AnalyzerPolicyInfo{
			{
				Name:             "no-framework-policy",
				EnforcementLevel: apitype.Advisory,
				Framework:        nil, // Explicitly nil
			},
		},
	}

	var capturedRequest *apitype.CreatePolicyPackRequest

	server := newMockServerRequestProcessor(200, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/policypacks") && r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			capturedRequest = &apitype.CreatePolicyPackRequest{}
			err = json.Unmarshal(body, capturedRequest)
			require.NoError(t, err)

			resp := apitype.CreatePolicyPackResponse{
				Version:   1,
				UploadURI: "http://" + r.Host + "/upload",
			}
			respJSON, err := json.Marshal(resp)
			require.NoError(t, err)
			return string(respJSON)
		}

		if strings.HasSuffix(r.URL.Path, "/upload") && r.Method == "PUT" {
			return ""
		}

		if strings.Contains(r.URL.Path, "/complete") && r.Method == "POST" {
			return ""
		}

		return ""
	})
	defer server.Close()

	client := newMockClient(server)
	archive := bytes.NewReader([]byte("mock-archive-data"))

	_, err := client.PublishPolicyPack(context.Background(), "test-org", analyzerInfo, archive)
	require.NoError(t, err)

	// Verify nil compliance framework is handled correctly
	require.NotNil(t, capturedRequest)
	require.Len(t, capturedRequest.Policies, 1)
	policy := capturedRequest.Policies[0]
	assert.Nil(t, policy.Framework)
}
