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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
		_, _, _, unauthorizedErr := unauthorizedClient.GetCLIVersionInfo(context.Background())

		assert.EqualError(t, unauthorizedErr, "this command requires logging in; try running `pulumi login` first")
	})
	t.Run("TestRateLimitError", func(t *testing.T) {
		t.Parallel()

		// test handling 429: Too Many Requests/rate-limit response
		rateLimitedServer := newMockServer(429, "rate-limit error")
		defer rateLimitedServer.Close()

		rateLimitedClient := newMockClient(rateLimitedServer)
		_, _, _, rateLimitErr := rateLimitedClient.GetCLIVersionInfo(context.Background())

		assert.EqualError(t, rateLimitErr, "pulumi service: request rate-limit exceeded")
	})
	t.Run("TestDefaultError", func(t *testing.T) {
		t.Parallel()

		// test handling non-standard error message
		defaultErrorServer := newMockServer(418, "I'm a teapot")
		defer defaultErrorServer.Close()

		defaultErrorClient := newMockClient(defaultErrorServer)
		_, _, _, defaultErrorErr := defaultErrorClient.GetCLIVersionInfo(context.Background())

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
	latestVersion, oldestWithoutWarning, latestDevVersion, err := versionClient.GetCLIVersionInfo(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, latestVersion.String(), "1.0.0")
	assert.Equal(t, oldestWithoutWarning.String(), "0.1.0")
	assert.Equal(t, latestDevVersion.String(), "1.0.0-11-gdeadbeef")
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

	// POST /import
	_, err := client.ImportStackDeployment(context.Background(), StackIdentifier{}, nil)
	assert.NoError(t, err)

	tok := updateTokenStaticSource("")

	// PATCH /checkpoint
	err = client.PatchUpdateCheckpoint(context.Background(), UpdateIdentifier{}, nil, tok)
	assert.NoError(t, err)

	// POST /events/batch
	err = client.RecordEngineEvents(context.Background(), UpdateIdentifier{}, apitype.EngineEventBatch{}, tok)
	assert.NoError(t, err)

	// POST /events/batch
	_, err = client.BulkDecryptValue(context.Background(), StackIdentifier{}, nil)
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
		UpdateIdentifier{}, sequenceNumber, indented, updateTokenStaticSource("token"))
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
		cfg := apitype.DeltaCheckpointUploadsConfigV1{
			CheckpointCutoffSizeBytes: 1024 * 1024 * 4,
		}
		cfgJSON, err := json.Marshal(cfg)
		require.NoError(t, err)
		actualResp := apitype.CapabilitiesResponse{
			Capabilities: []apitype.APICapabilityConfig{{
				Version:       3,
				Capability:    apitype.DeltaCheckpointUploads,
				Configuration: json.RawMessage(cfgJSON),
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
		assert.Len(t, resp.Capabilities, 1)
		assert.Equal(t, apitype.DeltaCheckpointUploads, resp.Capabilities[0].Capability)
		assert.Equal(t, `{"checkpointCutoffSizeBytes":4194304}`,
			string(resp.Capabilities[0].Configuration))
	})
}
