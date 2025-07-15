// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(statusCode int, message string) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(statusCode)
			_, err := rw.Write([]byte(message))
			if err != nil {
				return
			}
		}))
}

func newMockServerRequestProcessor(statusCode int, processor func(req *http.Request) string) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(statusCode)
			_, err := rw.Write([]byte(processor(req)))
			if err != nil {
				return
			}
		}))
}

func newMockClient(server *httptest.Server) *Client {
	httpClient := http.DefaultClient

	return &Client{
		apiURL:     server.URL,
		apiToken:   "",
		apiUser:    "",
		diag:       nil,
		httpClient: httpClient,
		restClient: &defaultRESTClient{
			client: &defaultHTTPClient{
				client: httpClient,
			},
		},
	}
}

func TestAPIErrorResponses(t *testing.T) {
	t.Parallel()

	t.Run("TestAuthError", func(t *testing.T) {
		t.Parallel()

		// check 401 error is handled
		unauthorizedServer := newMockServer(401, "401: Unauthorized")
		defer unauthorizedServer.Close()

		unauthorizedClient := newMockClient(unauthorizedServer)
		_, _, _, _, unauthorizedErr := unauthorizedClient.GetCLIVersionInfo(context.Background(), nil)

		assert.EqualError(t, unauthorizedErr, "this command requires logging in; try running `pulumi login` first")
	})
	t.Run("TestRateLimitError", func(t *testing.T) {
		t.Parallel()

		// test handling 429: Too Many Requests/rate-limit response
		rateLimitedServer := newMockServer(429, "rate-limit error")
		defer rateLimitedServer.Close()

		rateLimitedClient := newMockClient(rateLimitedServer)
		_, _, _, _, rateLimitErr := rateLimitedClient.GetCLIVersionInfo(context.Background(), nil)

		assert.EqualError(t, rateLimitErr, "pulumi service: request rate-limit exceeded")
	})
	t.Run("TestDefaultError", func(t *testing.T) {
		t.Parallel()

		// test handling non-standard error message
		defaultErrorServer := newMockServer(418, "I'm a teapot")
		defer defaultErrorServer.Close()

		defaultErrorClient := newMockClient(defaultErrorServer)
		_, _, _, _, defaultErrorErr := defaultErrorClient.GetCLIVersionInfo(context.Background(), nil)

		assert.Error(t, defaultErrorErr)
	})
}

func TestAPIVersionResponses(t *testing.T) {
	t.Parallel()

	versionServer := newMockServer(
		200,
		`{"latestVersion": "1.0.0", "oldestWithoutWarning": "0.1.0", "latestDevVersion": "1.0.0-11-gdeadbeef"}`,
	)
	defer versionServer.Close()

	versionClient := newMockClient(versionServer)
	latestVersion, oldestWithoutWarning, latestDevVersion, _, err := versionClient.GetCLIVersionInfo(
		context.Background(), nil,
	)

	assert.NoError(t, err)
	assert.Equal(t, latestVersion.String(), "1.0.0")
	assert.Equal(t, oldestWithoutWarning.String(), "0.1.0")
	assert.Equal(t, latestDevVersion.String(), "1.0.0-11-gdeadbeef")
}

func TestAPIVersionMetadataHeaders(t *testing.T) {
	t.Parallel()

	// Arrange.
	server := newMockServerRequestProcessor(200, func(req *http.Request) string {
		assert.Equal(t, "foo", req.Header.Get("X-Pulumi-First"))
		assert.Equal(t, "bar", req.Header.Get("X-Pulumi-Second"))
		assert.Empty(t, req.Header.Get("X-Pulumi-Third"))
		return `{"latestVersion": "1.0.0", "oldestWithoutWarning": "0.1.0", "latestDevVersion": "1.0.0-11-gdeadbeef"}`
	})
	defer server.Close()
	client := newMockClient(server)

	// Act.
	_, _, _, _, err := client.GetCLIVersionInfo(context.Background(), map[string]string{
		"First":  "foo",
		"Second": "bar",
	})

	// Assert.
	assert.NoError(t, err)
}

func TestGzip(t *testing.T) {
	t.Parallel()

	// test handling non-standard error message
	gzipCheckServer := newMockServerRequestProcessor(200, func(req *http.Request) string {
		assert.Equal(t, req.Header.Get("Content-Encoding"), "gzip")
		return "{}"
	})
	defer gzipCheckServer.Close()
	client := newMockClient(gzipCheckServer)

	identifier := StackIdentifier{
		Stack: tokens.MustParseStackName("stack"),
	}

	// POST /import
	_, err := client.ImportStackDeployment(context.Background(), identifier, nil)
	assert.NoError(t, err)

	tok := updateTokenStaticSource("")

	// PATCH /checkpoint
	err = client.PatchUpdateCheckpoint(context.Background(), UpdateIdentifier{
		StackIdentifier: identifier,
	}, nil, tok)
	assert.NoError(t, err)

	// POST /events/batch
	err = client.RecordEngineEvents(context.Background(), UpdateIdentifier{
		StackIdentifier: identifier,
	}, apitype.EngineEventBatch{}, tok)
	assert.NoError(t, err)

	// POST /events/batch
	_, err = client.BatchDecryptValue(context.Background(), identifier, nil)
	assert.NoError(t, err)
}

func TestPatchUpdateCheckpointVerbatimIndents(t *testing.T) {
	t.Parallel()

	deployment := apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			{URN: resource.URN("urn1")},
			{URN: resource.URN("urn2")},
		},
	}

	var serializedDeployment json.RawMessage
	serializedDeployment, err := json.Marshal(deployment)
	assert.NoError(t, err)

	untypedDeployment, err := json.Marshal(apitype.UntypedDeployment{
		Version:    3,
		Deployment: serializedDeployment,
	})
	assert.NoError(t, err)

	var request apitype.PatchUpdateVerbatimCheckpointRequest

	server := newMockServerRequestProcessor(200, func(req *http.Request) string {
		reader, err := gzip.NewReader(req.Body)
		assert.NoError(t, err)
		defer reader.Close()

		err = json.NewDecoder(reader).Decode(&request)
		assert.NoError(t, err)

		return "{}"
	})

	client := newMockClient(server)

	sequenceNumber := 1

	indented, err := marshalDeployment(&deployment)
	require.NoError(t, err)

	newlines := bytes.Count(indented, []byte{'\n'})

	err = client.PatchUpdateCheckpointVerbatim(context.Background(),
		UpdateIdentifier{
			StackIdentifier: StackIdentifier{
				Stack: tokens.MustParseStackName("stack"),
			},
		}, sequenceNumber, indented, updateTokenStaticSource("token"))
	assert.NoError(t, err)

	compacted := func(raw json.RawMessage) string {
		var buf bytes.Buffer
		err := json.Compact(&buf, []byte(raw))
		assert.NoError(t, err)
		return buf.String()
	}

	// It should have more than one line as json.Marshal would produce.
	assert.Equal(t, newlines+1, len(strings.Split(string(request.UntypedDeployment), "\n")))

	// Compacting should recover the same form as json.Marshal would produce.
	assert.Equal(t, string(untypedDeployment), compacted(request.UntypedDeployment))
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	t.Run("legacy-service-404", func(t *testing.T) {
		t.Parallel()
		s := newMockServer(404, "NOT FOUND")
		defer s.Close()

		c := newMockClient(s)
		resp, err := c.GetCapabilities(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Empty(t, resp.Capabilities)
	})
	t.Run("updated-service-with-delta-checkpoint-capability", func(t *testing.T) {
		t.Parallel()
		cfg := apitype.DeltaCheckpointUploadsConfigV2{
			CheckpointCutoffSizeBytes: 1024 * 1024 * 4,
		}
		cfgJSON, err := json.Marshal(cfg)
		require.NoError(t, err)
		actualResp := apitype.CapabilitiesResponse{
			Capabilities: []apitype.APICapabilityConfig{{
				Version:       3,
				Capability:    apitype.DeltaCheckpointUploads,
				Configuration: json.RawMessage(cfgJSON),
			}, {
				Capability: apitype.BatchEncrypt,
			}},
		}
		respJSON, err := json.Marshal(actualResp)
		require.NoError(t, err)
		s := newMockServer(200, string(respJSON))
		defer s.Close()

		c := newMockClient(s)
		resp, err := c.GetCapabilities(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Capabilities, 2)
		assert.Equal(t, apitype.DeltaCheckpointUploads, resp.Capabilities[0].Capability)
		assert.Equal(t, `{"checkpointCutoffSizeBytes":4194304}`,
			string(resp.Capabilities[0].Configuration))
		assert.Equal(t, resp.Capabilities[1].Capability, apitype.BatchEncrypt)

		parsed, err := resp.Parse()
		require.NoError(t, err)
		assert.Equal(t, parsed.DeltaCheckpointUpdates, &cfg)
		assert.True(t, parsed.BatchEncryption)
	})
}

func TestDeploymentSettingsApi(t *testing.T) {
	t.Parallel()
	t.Run("get-stack-deployment-settings", func(t *testing.T) {
		t.Parallel()

		payload := `{
    "sourceContext": {
        "git": {
            "repoUrl": "git@github.com:pulumi/test-repo.git",
            "branch": "main",
            "repoDir": ".",
            "gitAuth": {
                "basicAuth": {
                    "userName": "jdoe",
                    "password": {
                        "secret": "[secret]",
                        "ciphertext": "AAABAMcGtHDraogfM3Qk4WyaNp3F/syk2cjHPQTb6Hu6ps8="
                    }
                }
            }
        }
    },
    "operationContext": {
        "oidc": {
            "aws": {
                "duration": "1h0m0s",
                "policyArns": [
                    "policy:arn"
                ],
                "roleArn": "the_role",
                "sessionName": "the_session_name"
            }
        },
        "options": {
            "skipIntermediateDeployments": true
        }
    },
    "agentPoolID": "51035bee-a4d6-4b63-9ff6-418775c5da8d"
}`

		s := newMockServerRequestProcessor(200, func(req *http.Request) string {
			assert.Equal(t, req.RequestURI, "/api/stacks/owner/project/stack/deployments/settings")
			return payload
		})
		defer s.Close()

		c := newMockClient(s)
		stack, _ := tokens.ParseStackName("stack")
		resp, err := c.GetStackDeploymentSettings(context.Background(), StackIdentifier{
			Owner:   "owner",
			Project: "project",
			Stack:   stack,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.SourceContext)
		assert.NotNil(t, resp.SourceContext.Git)
		assert.Equal(t, "main", resp.SourceContext.Git.Branch)
		assert.Equal(t, "git@github.com:pulumi/test-repo.git", resp.SourceContext.Git.RepoURL)
		assert.Equal(t, ".", resp.SourceContext.Git.RepoDir)
		assert.NotNil(t, resp.SourceContext.Git.GitAuth)
		assert.NotNil(t, resp.SourceContext.Git.GitAuth.BasicAuth)
		assert.NotNil(t, resp.SourceContext.Git.GitAuth.BasicAuth.UserName)
		assert.Equal(t, "jdoe", resp.SourceContext.Git.GitAuth.BasicAuth.UserName.Value)
		assert.NotNil(t, resp.SourceContext.Git.GitAuth.BasicAuth.Password)
		assert.Equal(t, "AAABAMcGtHDraogfM3Qk4WyaNp3F/syk2cjHPQTb6Hu6ps8=",
			resp.SourceContext.Git.GitAuth.BasicAuth.Password.Ciphertext)
		assert.NotNil(t, resp.Operation)
		assert.NotNil(t, resp.Operation.Options)
		assert.True(t, resp.Operation.Options.SkipIntermediateDeployments)
		assert.False(t, resp.Operation.Options.DeleteAfterDestroy)
		assert.False(t, resp.Operation.Options.RemediateIfDriftDetected)
		assert.False(t, resp.Operation.Options.SkipInstallDependencies)
		assert.NotNil(t, resp.Operation.OIDC)
		assert.Nil(t, resp.Operation.OIDC.Azure)
		assert.Nil(t, resp.Operation.OIDC.GCP)
		assert.NotNil(t, resp.Operation.OIDC.AWS)
		assert.Equal(t, "the_session_name", resp.Operation.OIDC.AWS.SessionName)
		assert.Equal(t, "the_role", resp.Operation.OIDC.AWS.RoleARN)
		duration, _ := time.ParseDuration("1h0m0s")
		assert.Equal(t, apitype.DeploymentDuration(duration), resp.Operation.OIDC.AWS.Duration)
		assert.Equal(t, []string{"policy:arn"}, resp.Operation.OIDC.AWS.PolicyARNs)
		assert.Equal(t, "51035bee-a4d6-4b63-9ff6-418775c5da8d", *resp.AgentPoolID)
	})
}

func TestListOrgTemplates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("with-templates", func(t *testing.T) {
		t.Parallel()

		s := newMockServerRequestProcessor(200, func(req *http.Request) string {
			const s1 = `[{"sourceName":"source1","name":"some-name","sourceURL":"example.com"}]`
			return `{"orgHasTemplates":true,"templates":{"source1":` + s1 + `}}`
		})
		defer s.Close()
		c := newMockClient(s)

		actual, err := c.ListOrgTemplates(ctx, "some-org")
		require.NoError(t, err)

		assert.Equal(t, apitype.ListOrgTemplatesResponse{
			OrgHasTemplates: true,
			Templates: map[string][]*apitype.PulumiTemplateRemote{
				"source1": {
					{SourceName: "source1", Name: "some-name", TemplateURL: "example.com"},
				},
			},
		}, actual)
	})

	t.Run("org-with-no-templates", func(t *testing.T) {
		t.Parallel()

		s := newMockServerRequestProcessor(200, func(req *http.Request) string {
			return `{"orgHasTemplates":true}`
		})
		defer s.Close()
		c := newMockClient(s)

		actual, err := c.ListOrgTemplates(ctx, "some-org")
		require.NoError(t, err)

		assert.Equal(t, apitype.ListOrgTemplatesResponse{
			OrgHasTemplates: true,
		}, actual)
	})

	t.Run("has-access-error", func(t *testing.T) {
		t.Parallel()

		s := newMockServerRequestProcessor(200, func(req *http.Request) string {
			return `{"hasAccessError":true}`
		})
		defer s.Close()
		c := newMockClient(s)

		actual, err := c.ListOrgTemplates(ctx, "some-org")
		require.NoError(t, err)

		assert.Equal(t, apitype.ListOrgTemplatesResponse{
			HasAccessError: true,
		}, actual)
	})
}

func TestGetDefaultOrg(t *testing.T) {
	t.Parallel()
	t.Run("legacy-service-404", func(t *testing.T) {
		t.Parallel()
		// GIVEN
		s := newMockServer(404, "NOT FOUND")
		defer s.Close()

		// WHEN
		c := newMockClient(s)
		resp, err := c.GetDefaultOrg(context.Background())

		// THEN
		// We should gracefully handle the 404
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Empty(t, resp.GitHubLogin)
	})
}

func TestGetPackage(t *testing.T) {
	t.Parallel()

	const metadataJSON = `{
  "name": "my-package",
  "publisher": "my-publisher",
  "source": "my-source",
  "version": "1.0.0",
  "title": "Example Package",
  "description": "This is an example package.",
  "logoUrl": "https://example.com/logo.png",
  "repoUrl": "https://github.com/example/package",
  "category": "utilities",
  "isFeatured": true,
  "packageTypes": ["native", "component"],
  "packageStatus": "ga",
  "readmeURL": "https://example.com/readme",
  "schemaURL": "https://example.com/schema",
  "pluginDownloadURL": "https://example.com/download",
  "createdAt": "2023-10-01T12:00:00Z",
  "visibility": "public"
}`

	metadata := func() apitype.PackageMetadata {
		return apitype.PackageMetadata{
			Name:              "my-package",
			Publisher:         "my-publisher",
			Source:            "my-source",
			Version:           semver.Version{Major: 1},
			Title:             "Example Package",
			Description:       "This is an example package.",
			LogoURL:           "https://example.com/logo.png",
			RepoURL:           "https://github.com/example/package",
			Category:          "utilities",
			IsFeatured:        true,
			PackageTypes:      []apitype.PackageType{"native", "component"},
			PackageStatus:     apitype.PackageStatusGA,
			ReadmeURL:         "https://example.com/readme",
			SchemaURL:         "https://example.com/schema",
			PluginDownloadURL: "https://example.com/download",
			CreatedAt:         time.Date(2023, time.October, 1, 12, 0, 0, 0, time.UTC),
			Visibility:        apitype.VisibilityPublic,
		}
	}

	t.Run("latest-latest", func(t *testing.T) {
		t.Parallel()
		s := newMockServerRequestProcessor(200, func(req *http.Request) string {
			assert.Contains(t, req.URL.String(), "my-source/my-publisher/my-package/versions/latest")
			return metadataJSON
		})
		defer s.Close()

		c := newMockClient(s)

		resp, err := c.GetPackage(context.Background(), "my-source", "my-publisher", "my-package", nil)
		require.NoError(t, err)
		assert.Equal(t, metadata(), resp)
	})

	t.Run("404", func(t *testing.T) {
		t.Parallel()
		s := newMockServer(404, ``)
		defer s.Close()

		c := newMockClient(s)

		_, err := c.GetPackage(context.Background(), "my-source", "my-publisher", "my-package", nil)
		var apiError *apitype.ErrorResponse
		require.ErrorAs(t, err, &apiError, "actual error type %T", err)
		assert.Equal(t, 404, apiError.Code)
	})

	t.Run("specific-version", func(t *testing.T) {
		t.Parallel()
		s := newMockServerRequestProcessor(200, func(req *http.Request) string {
			assert.Contains(t, req.URL.String(), "my-source/my-publisher/my-package/versions/1.0.0")
			return metadataJSON
		})
		defer s.Close()

		c := newMockClient(s)

		resp, err := c.GetPackage(context.Background(), "my-source", "my-publisher", "my-package", &semver.Version{
			Major: 1,
		})
		require.NoError(t, err)
		assert.Equal(t, metadata(), resp)
	})
}

func TestListPackages(t *testing.T) {
	t.Parallel()

	t.Run("no-continuation-token", func(t *testing.T) {
		t.Parallel()

		// Create a mock response with package metadata
		expectedPackages := []apitype.PackageMetadata{
			{
				Name:          "my-package-1",
				Publisher:     "my-publisher",
				Source:        "my-source",
				Version:       semver.Version{Major: 1},
				PackageStatus: apitype.PackageStatusGA,
				Visibility:    apitype.VisibilityPrivate,
			},
			{
				Name:          "my-package-2",
				Publisher:     "my-publisher",
				Source:        "my-source",
				Version:       semver.Version{Major: 2},
				PackageStatus: apitype.PackageStatusGA,
				Visibility:    apitype.VisibilityPrivate,
			},
		}

		mockResponse := apitype.ListPackagesResponse{
			Packages: expectedPackages,
		}

		// Set up mock server
		mockServer := newMockServerRequestProcessor(200, func(req *http.Request) string {
			assert.Contains(t, req.URL.String(), "/api/preview/registry/packages?limit=499")
			assert.Equal(t, "GET", req.Method)

			data, err := json.Marshal(mockResponse)
			require.NoError(t, err)
			return string(data)
		})
		defer mockServer.Close()

		mockClient := newMockClient(mockServer)

		// Call ListPackages and collect results
		searchName := "my-package"
		searchResults := []apitype.PackageMetadata{}
		for pkg, err := range mockClient.ListPackages(context.Background(), &searchName) {
			require.NoError(t, err)
			searchResults = append(searchResults, pkg)
		}
		assert.Equal(t, expectedPackages, searchResults)
	})

	t.Run("with-continuation-token", func(t *testing.T) {
		t.Parallel()

		// First page response
		firstPagePackages := []apitype.PackageMetadata{
			{
				Name:          "my-package-1",
				Publisher:     "my-publisher",
				Source:        "my-source",
				Version:       semver.Version{Major: 1},
				PackageStatus: apitype.PackageStatusGA,
				Visibility:    apitype.VisibilityPrivate,
			},
		}

		secondPagePackages := []apitype.PackageMetadata{
			{
				Name:          "my-package-2",
				Publisher:     "my-publisher",
				Source:        "my-source",
				Version:       semver.Version{Major: 2},
				PackageStatus: apitype.PackageStatusGA,
				Visibility:    apitype.VisibilityPrivate,
			},
		}

		thirdPagePackages := []apitype.PackageMetadata{
			{
				Name:          "my-package-3",
				Publisher:     "my-publisher",
				Source:        "my-source",
				Version:       semver.Version{Major: 3},
				PackageStatus: apitype.PackageStatusGA,
				Visibility:    apitype.VisibilityPrivate,
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
				assert.Equal(t, "/api/preview/registry/packages?limit=499&name=my-package", req.URL.String())
				assert.NotContains(t, "continuationToken", req.URL.String())

				responseData, err = json.Marshal(apitype.ListPackagesResponse{
					Packages:          firstPagePackages,
					ContinuationToken: ptr("next-page-token-1"),
				})
				require.NoError(t, err)
			case 1:
				assert.Equal(t,
					"/api/preview/registry/packages?limit=499&name=my-package&continuationToken=next-page-token-1",
					req.URL.String())

				responseData, err = json.Marshal(apitype.ListPackagesResponse{
					Packages:          secondPagePackages,
					ContinuationToken: ptr("next-page-token-2"),
				})
				require.NoError(t, err)
			case 2:
				assert.Equal(t,
					"/api/preview/registry/packages?limit=499&name=my-package&continuationToken=next-page-token-2",
					req.URL.String())

				responseData, err = json.Marshal(apitype.ListPackagesResponse{
					Packages: thirdPagePackages,
				})
				require.NoError(t, err)
			}

			requestCount++
			return string(responseData)
		})
		defer mockServer.Close()

		mockClient := newMockClient(mockServer)

		searchName := "my-package"
		searchResults := []apitype.PackageMetadata{}
		for pkg, err := range mockClient.ListPackages(context.Background(), &searchName) {
			require.NoError(t, err)
			searchResults = append(searchResults, pkg)
		}

		expectedPackages := append(append(firstPagePackages, secondPagePackages...), thirdPagePackages...)
		assert.Equal(t, expectedPackages, searchResults)
		assert.Equal(t, 3, requestCount) // Ensure both requests were made
	})
}

func ptr[T any](v T) *T { return &v }

func TestCallCopilot(t *testing.T) {
	t.Parallel()

	t.Run("StatusNoContent", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns 204 No Content, it should return an empty string without error
		noContentServer := newMockServer(http.StatusNoContent, "")
		defer noContentServer.Close()

		client := newMockClient(noContentServer)
		response, err := client.callCopilot(context.Background(), map[string]string{"test": "data"})

		require.NoError(t, err)
		require.Empty(t, response)
	})

	t.Run("StatusPaymentRequired", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns 402 Payment Required (usage limit), it should return an error with the body message
		usageLimitServer := newMockServer(http.StatusPaymentRequired, "Usage limit reached")
		defer usageLimitServer.Close()

		client := newMockClient(usageLimitServer)
		response, err := client.callCopilot(context.Background(), map[string]string{"test": "data"})

		require.EqualError(t, err, "Usage limit reached")
		require.Empty(t, response)
	})

	t.Run("OtherErrorStatus", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns other error status codes, it should return an error with the body message
		errorServer := newMockServer(http.StatusInternalServerError, "Internal server error")
		defer errorServer.Close()

		client := newMockClient(errorServer)
		response, err := client.callCopilot(context.Background(), map[string]string{"test": "data"})

		require.EqualError(t, err, "Internal server error")
		require.Empty(t, response)
	})

	t.Run("EmptyErrorBody", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns error status with empty body, it should return a generic error
		emptyBodyServer := newMockServer(http.StatusBadRequest, "")
		defer emptyBodyServer.Close()

		client := newMockClient(emptyBodyServer)
		response, err := client.callCopilot(context.Background(), map[string]string{"test": "data"})

		require.ErrorContains(t, err, "Copilot API returned error status: 400")
		require.Empty(t, response)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns a non-JSON response with status 200, it should return an error
		invalidJSONServer := newMockServer(http.StatusOK, "This is not JSON")
		defer invalidJSONServer.Close()

		client := newMockClient(invalidJSONServer)
		response, err := client.callCopilot(context.Background(), map[string]string{"test": "data"})

		require.EqualError(t, err, "unable to parse Copilot response: This is not JSON")
		require.Empty(t, response)
	})

	t.Run("ValidJSON", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns a valid JSON response, it should process it correctly
		validJSONServer := newMockServer(http.StatusOK, `{
			"messages": [
				{
					"role": "assistant",
					"kind": "response",
					"content": "\"This is a valid response\""
				}
			]
		}`)
		defer validJSONServer.Close()

		client := newMockClient(validJSONServer)
		response, err := client.callCopilot(context.Background(), map[string]string{"test": "data"})

		require.NoError(t, err)
		require.Equal(t, "\"This is a valid response\"", response)
	})

	t.Run("JSONWithError", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns a JSON response with an error field, it should return that error
		errorJSONServer := newMockServer(http.StatusOK, `{
			"error": "API error message",
			"details": "Detailed error information"
		}`)
		defer errorJSONServer.Close()

		client := newMockClient(errorJSONServer)
		response, err := client.callCopilot(context.Background(), map[string]string{"test": "data"})

		require.EqualError(t, err, "copilot API error: API error message\nDetailed error information")
		require.Empty(t, response)
	})
}
