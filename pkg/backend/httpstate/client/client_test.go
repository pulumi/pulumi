// Copyright 2016, Pulumi Corporation.
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
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
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

func newHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		// Copy of http.DefaultTransport settings except Proxy
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func newMockClient(server *httptest.Server) *Client {
	httpClient := newHTTPClient()

	return &Client{
		apiURL:   server.URL,
		apiToken: apiAccessToken(""),
		apiUser:  "",
		diag:     nil,
		restClient: &defaultRESTClient{
			client: &defaultHTTPClient{
				client: httpClient,
			},
		},
	}
}

func TestSignupAgent(t *testing.T) {
	t.Parallel()

	validUntil := time.Now().UTC().Truncate(time.Second)
	expiresAt := validUntil.Add(-time.Hour)
	const challengeID = "challenge-1"
	const challengeData = "v1:abcdef:8"
	var requests []agentSignupRequest
	var gotPaths, gotMethods, gotAuths []string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		gotPaths = append(gotPaths, req.URL.Path)
		gotMethods = append(gotMethods, req.Method)
		gotAuths = append(gotAuths, req.Header.Get("Authorization"))
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		switch len(gotMethods) {
		case 1:
			assert.Equal(t, http.MethodGet, req.Method)
			assert.Empty(t, body)
			err = json.NewEncoder(rw).Encode(AgentSignupChallenge{
				ChallengeID:   challengeID,
				ChallengeData: challengeData,
			})
			require.NoError(t, err)
		case 2:
			assert.Equal(t, http.MethodPost, req.Method)
			var signupReq agentSignupRequest
			require.NoError(t, json.Unmarshal(body, &signupReq))
			requests = append(requests, signupReq)
			assert.Equal(t, challengeID, signupReq.ChallengeID)
			assert.Equal(t, "codex", signupReq.AgentName)
			assert.Equal(t, "gpt-test", signupReq.AgentModel)
			assert.GreaterOrEqual(t, signupReq.ChallengeSolveDurationMS, int64(0))
			require.NoError(t, verifyAgentSignupChallenge(challengeData, signupReq.ChallengeResult))
			err = json.NewEncoder(rw).Encode(AgentSignupResponse{
				AccessToken:           "agent-token",
				AccessTokenValidUntil: expiresAt,
				ClaimToken:            "abc123",
				ClaimTokenValidUntil:  validUntil,
			})
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected signup request %d", len(gotMethods))
		}
	}))
	defer server.Close()

	resp, err := NewClient(server.URL, "", true, nil).SignupAgent(t.Context(), agentdetect.Metadata{
		Name:  "codex",
		Model: "gpt-test",
	})
	require.NoError(t, err)

	assert.Equal(t, []string{http.MethodGet, http.MethodPost}, gotMethods)
	assert.Equal(t, []string{"/api/agents/signup", "/api/agents/signup"}, gotPaths)
	assert.Equal(t, []string{"", ""}, gotAuths)
	require.Len(t, requests, 1)
	assert.Equal(t, "agent-token", resp.AccessToken)
	assert.Equal(t, "abc123", resp.ClaimToken)
	assert.True(t, resp.AccessTokenValidUntil.Equal(expiresAt))
	assert.True(t, resp.ClaimTokenValidUntil.Equal(validUntil))
}

func TestSignupAgentRequiresChallengeData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/api/agents/signup", req.URL.Path)
		err := json.NewEncoder(rw).Encode(AgentSignupChallenge{})
		require.NoError(t, err)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "", true, nil).SignupAgent(t.Context(), agentdetect.Metadata{Name: "codex"})
	require.ErrorContains(t, err, "signup response did not include challenge data")
}

func TestSignupAgentReturnsInitialSignupError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/api/agents/signup", req.URL.Path)
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "", true, nil).SignupAgent(t.Context(), agentdetect.Metadata{Name: "codex"})
	require.Error(t, err)
}

func TestSignupAgentReturnsChallengeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/api/agents/signup", req.URL.Path)
		err := json.NewEncoder(rw).Encode(AgentSignupChallenge{
			ChallengeID:   "challenge-1",
			ChallengeData: "v2:abcdef:8",
		})
		require.NoError(t, err)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "", true, nil).SignupAgent(t.Context(), agentdetect.Metadata{Name: "codex"})
	require.ErrorContains(t, err, "invalid challenge data")
}

func TestSignupAgentReturnsFinalSignupError(t *testing.T) {
	t.Parallel()

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/api/agents/signup", req.URL.Path)
		requestCount++
		if requestCount == 1 {
			assert.Equal(t, http.MethodGet, req.Method)
			err := json.NewEncoder(rw).Encode(AgentSignupChallenge{
				ChallengeID:   "challenge-1",
				ChallengeData: "v1:abcdef:8",
			})
			require.NoError(t, err)
			return
		}
		assert.Equal(t, http.MethodPost, req.Method)
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "", true, nil).SignupAgent(t.Context(), agentdetect.Metadata{Name: "codex"})
	require.Error(t, err)
	assert.Equal(t, 2, requestCount)
}

func TestSignupAgentRequiresFinalSignupFields(t *testing.T) {
	t.Parallel()

	validUntil := time.Now().UTC().Truncate(time.Second)
	tests := []struct {
		name      string
		response  AgentSignupResponse
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "missing access token",
			response: AgentSignupResponse{
				AccessTokenValidUntil: validUntil,
				ClaimToken:            "claim-token",
				ClaimTokenValidUntil:  validUntil,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "signup response did not include an access token", i...)
			},
		},
		{
			name: "missing access token valid until",
			response: AgentSignupResponse{
				AccessToken:          "agent-token",
				ClaimToken:           "claim-token",
				ClaimTokenValidUntil: validUntil,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "signup response did not include accessTokenValidUntil", i...)
			},
		},
		{
			name: "missing claim token",
			response: AgentSignupResponse{
				AccessToken:           "agent-token",
				AccessTokenValidUntil: validUntil,
				ClaimTokenValidUntil:  validUntil,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "signup response did not include a claim token", i...)
			},
		},
		{
			name: "missing claim token valid until",
			response: AgentSignupResponse{
				AccessToken:           "agent-token",
				AccessTokenValidUntil: validUntil,
				ClaimToken:            "claim-token",
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "signup response did not include claimTokenValidUntil", i...)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestCount int
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				assert.Equal(t, "/api/agents/signup", req.URL.Path)
				requestCount++
				if requestCount == 1 {
					assert.Equal(t, http.MethodGet, req.Method)
					err := json.NewEncoder(rw).Encode(AgentSignupChallenge{
						ChallengeID:   "challenge-1",
						ChallengeData: "v1:abcdef:8",
					})
					require.NoError(t, err)
					return
				}
				assert.Equal(t, http.MethodPost, req.Method)
				err := json.NewEncoder(rw).Encode(tt.response)
				require.NoError(t, err)
			}))
			defer server.Close()

			_, err := NewClient(server.URL, "", true, nil).SignupAgent(
				t.Context(),
				agentdetect.Metadata{Name: "codex"})
			tt.assertErr(t, err)
			assert.Equal(t, 2, requestCount)
		})
	}
}

func TestSolveAgentSignupChallengeHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := solveAgentSignupChallenge(ctx, "v1:abcdef:256")
	require.ErrorIs(t, err, context.Canceled)
}

func TestValidateAgentClaim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		statusCode    int
		wantClaimable bool
		wantErr       bool
	}{
		{
			name:          "claimable",
			statusCode:    http.StatusOK,
			wantClaimable: true,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotAuth string
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				assert.Equal(t, http.MethodGet, req.Method)
				assert.Equal(t, "/api/agents/signup/validate/claim-token", req.URL.Path)
				gotAuth = req.Header.Get("Authorization")
				rw.WriteHeader(tt.statusCode)
				if tt.statusCode >= 400 {
					err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{
						Code:    tt.statusCode,
						Message: "validation failed",
					})
					require.NoError(t, err)
				}
			}))
			defer server.Close()

			claimable, err := NewClient(server.URL, "", true, nil).
				ValidateAgentClaim(t.Context(), "claim-token")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantClaimable, claimable)
			assert.Empty(t, gotAuth)
		})
	}
}

func TestParseAgentSignupChallengeDifficulty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		data      string
		want      int
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "valid",
			data:      "v1:abcdef:8",
			want:      8,
			assertErr: require.NoError,
		},
		{
			name:      "invalid version",
			data:      "v2:abcdef:8",
			assertErr: require.Error,
		},
		{
			name:      "missing part",
			data:      "v1:abcdef",
			assertErr: require.Error,
		},
		{
			name:      "invalid difficulty",
			data:      "v1:abcdef:nope",
			assertErr: require.Error,
		},
		{
			name:      "zero difficulty",
			data:      "v1:abcdef:0",
			assertErr: require.Error,
		},
		{
			name:      "excessive difficulty",
			data:      "v1:abcdef:257",
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseAgentSignupChallengeDifficulty(tt.data)
			tt.assertErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLeadingZeroBits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, leadingZeroBits([]byte{0xff}))
	assert.Equal(t, 4, leadingZeroBits([]byte{0x0f}))
	assert.Equal(t, 12, leadingZeroBits([]byte{0x00, 0x0f}))
	assert.Equal(t, 16, leadingZeroBits([]byte{0x00, 0x00}))
}

func verifyAgentSignupChallenge(data, result string) error {
	difficulty, err := parseAgentSignupChallengeDifficulty(data)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(data + ":" + result))
	if leadingZeroBits(hash[:]) < difficulty {
		return errors.New("insufficient work for challenge")
	}
	return nil
}

func TestAPIErrorResponses(t *testing.T) {
	t.Parallel()

	t.Run("TestAuthError", func(t *testing.T) {
		t.Parallel()

		// check 401 error is handled
		unauthorizedServer := newMockServer(401, "401: Unauthorized")
		defer unauthorizedServer.Close()

		unauthorizedClient := newMockClient(unauthorizedServer)
		_, _, _, unauthorizedErr := unauthorizedClient.GetCLIVersionInfo(t.Context(), nil)

		assert.EqualError(t, unauthorizedErr, "this command requires logging in; try running `pulumi login` first")
	})
	t.Run("TestRateLimitError", func(t *testing.T) {
		t.Parallel()

		// test handling 429: Too Many Requests/rate-limit response
		rateLimitedServer := newMockServer(429, "rate-limit error")
		defer rateLimitedServer.Close()

		rateLimitedClient := newMockClient(rateLimitedServer)
		_, _, _, rateLimitErr := rateLimitedClient.GetCLIVersionInfo(t.Context(), nil)

		assert.EqualError(t, rateLimitErr, "pulumi service: request rate-limit exceeded")
	})
	t.Run("TestDefaultError", func(t *testing.T) {
		t.Parallel()

		// test handling non-standard error message
		defaultErrorServer := newMockServer(418, "I'm a teapot")
		defer defaultErrorServer.Close()

		defaultErrorClient := newMockClient(defaultErrorServer)
		_, _, _, defaultErrorErr := defaultErrorClient.GetCLIVersionInfo(t.Context(), nil)

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
	latestVersion, oldestWithoutWarning, latestDevVersion, err := versionClient.GetCLIVersionInfo(
		t.Context(), nil,
	)

	require.NoError(t, err)
	assert.Equal(t, latestVersion.String(), "1.0.0")
	assert.Equal(t, oldestWithoutWarning.String(), "0.1.0")
	assert.Equal(t, latestDevVersion.String(), "1.0.0-11-gdeadbeef")
}

func TestAcceptAPIVersionHeader(t *testing.T) {
	t.Parallel()

	// This test pins the Accept header value the CLI sends. If this fails
	// because you bumped `currentAPIVersion`, also append a row to the version
	// history table in api.go (and update the matching version block in
	// pulumi-service `cmd/service/api/rest/request.go`).
	var handled atomic.Bool
	server := newMockServerRequestProcessor(200, func(req *http.Request) string {
		handled.Store(true)
		assert.Equal(t, "application/vnd.pulumi+9", req.Header.Get("Accept"))
		return `{"latestVersion": "1.0.0", "oldestWithoutWarning": "0.1.0", "latestDevVersion": "1.0.0"}`
	})
	defer server.Close()
	client := newMockClient(server)

	_, _, _, err := client.GetCLIVersionInfo(t.Context(), nil)
	require.NoError(t, err)
	assert.True(t, handled.Load(), "mock server handler did not run")
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
	_, _, _, err := client.GetCLIVersionInfo(t.Context(), map[string]string{
		"First":  "foo",
		"Second": "bar",
	})

	// Assert.
	require.NoError(t, err)
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
	_, err := client.ImportStackDeployment(t.Context(), identifier, nil)
	require.NoError(t, err)

	tok := updateTokenStaticSource("")

	// PATCH /checkpoint
	err = client.PatchUpdateCheckpoint(t.Context(), UpdateIdentifier{
		StackIdentifier: identifier,
	}, &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage("{}"),
	}, tok)
	require.NoError(t, err)

	// POST /events/batch
	err = client.RecordEngineEvents(t.Context(), UpdateIdentifier{
		StackIdentifier: identifier,
	}, apitype.EngineEventBatch{}, tok)
	require.NoError(t, err)

	// POST /events/batch
	_, err = client.BatchDecryptValue(t.Context(), identifier, nil)
	require.NoError(t, err)
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
	require.NoError(t, err)

	untypedDeployment, err := json.Marshal(apitype.UntypedDeployment{
		Version:    3,
		Deployment: serializedDeployment,
	})
	require.NoError(t, err)

	var request apitype.PatchUpdateVerbatimCheckpointRequest

	server := newMockServerRequestProcessor(200, func(req *http.Request) string {
		reader, err := gzip.NewReader(req.Body)
		require.NoError(t, err)
		defer reader.Close()

		err = json.NewDecoder(reader).Decode(&request)
		require.NoError(t, err)

		return "{}"
	})

	client := newMockClient(server)

	sequenceNumber := 1

	indented, err := marshalDeployment(&deployment)
	require.NoError(t, err)

	newlines := bytes.Count(indented, []byte{'\n'})

	err = client.PatchUpdateCheckpointVerbatim(t.Context(),
		UpdateIdentifier{
			StackIdentifier: StackIdentifier{
				Stack: tokens.MustParseStackName("stack"),
			},
		}, sequenceNumber, indented, 3, updateTokenStaticSource("token"))
	require.NoError(t, err)

	compacted := func(raw json.RawMessage) string {
		var buf bytes.Buffer
		err := json.Compact(&buf, []byte(raw))
		require.NoError(t, err)
		return buf.String()
	}

	// It should have more than one line as json.Marshal would produce.
	require.Len(t, strings.Split(string(request.UntypedDeployment), "\n"), newlines+1)

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
		resp, err := c.GetCapabilities(t.Context())
		require.NoError(t, err)
		require.NotNil(t, resp)
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
		resp, err := c.GetCapabilities(t.Context())
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, resp.Capabilities, 2)
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
		resp, err := c.GetStackDeploymentSettings(t.Context(), StackIdentifier{
			Owner:   "owner",
			Project: "project",
			Stack:   stack,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.SourceContext)
		require.NotNil(t, resp.SourceContext.Git)
		assert.Equal(t, "main", resp.SourceContext.Git.Branch)
		assert.Equal(t, "git@github.com:pulumi/test-repo.git", resp.SourceContext.Git.RepoURL)
		assert.Equal(t, ".", resp.SourceContext.Git.RepoDir)
		require.NotNil(t, resp.SourceContext.Git.GitAuth)
		require.NotNil(t, resp.SourceContext.Git.GitAuth.BasicAuth)
		require.NotNil(t, resp.SourceContext.Git.GitAuth.BasicAuth.UserName)
		assert.Equal(t, "jdoe", resp.SourceContext.Git.GitAuth.BasicAuth.UserName.Value)
		require.NotNil(t, resp.SourceContext.Git.GitAuth.BasicAuth.Password)
		assert.Equal(t, "AAABAMcGtHDraogfM3Qk4WyaNp3F/syk2cjHPQTb6Hu6ps8=",
			resp.SourceContext.Git.GitAuth.BasicAuth.Password.Ciphertext)
		require.NotNil(t, resp.Operation)
		require.NotNil(t, resp.Operation.Options)
		assert.True(t, resp.Operation.Options.SkipIntermediateDeployments)
		assert.False(t, resp.Operation.Options.DeleteAfterDestroy)
		assert.False(t, resp.Operation.Options.RemediateIfDriftDetected)
		assert.False(t, resp.Operation.Options.SkipInstallDependencies)
		require.NotNil(t, resp.Operation.OIDC)
		assert.Nil(t, resp.Operation.OIDC.Azure)
		assert.Nil(t, resp.Operation.OIDC.GCP)
		require.NotNil(t, resp.Operation.OIDC.AWS)
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
	ctx := t.Context()

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
		resp, err := c.GetDefaultOrg(t.Context())

		// THEN
		// We should gracefully handle the 404
		require.NoError(t, err)
		require.NotNil(t, resp)
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

		resp, err := c.GetPackage(t.Context(), "my-source", "my-publisher", "my-package", nil)
		require.NoError(t, err)
		assert.Equal(t, metadata(), resp)
	})

	t.Run("404", func(t *testing.T) {
		t.Parallel()
		s := newMockServer(404, ``)
		defer s.Close()

		c := newMockClient(s)

		_, err := c.GetPackage(t.Context(), "my-source", "my-publisher", "my-package", nil)
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

		resp, err := c.GetPackage(t.Context(), "my-source", "my-publisher", "my-package", &semver.Version{
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
			assert.Contains(t, req.URL.String(), "/api/registry/packages?limit=499")
			assert.Equal(t, "GET", req.Method)

			data, err := json.Marshal(mockResponse)
			require.NoError(t, err)
			return string(data)
		})
		defer mockServer.Close()

		mockClient := newMockClient(mockServer)

		// Call ListPackages and collect results
		searchName := "my-package"
		//nolint:prealloc // capacity unknown ahead of time
		searchResults := []apitype.PackageMetadata{}
		for pkg, err := range mockClient.ListPackages(t.Context(), &searchName) {
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
				assert.Equal(t, "/api/registry/packages?limit=499&name=my-package", req.URL.String())
				assert.NotContains(t, "continuationToken", req.URL.String())

				responseData, err = json.Marshal(apitype.ListPackagesResponse{
					Packages:          firstPagePackages,
					ContinuationToken: ptr("next-page-token-1"),
				})
				require.NoError(t, err)
			case 1:
				assert.Equal(t,
					"/api/registry/packages?limit=499&name=my-package&continuationToken=next-page-token-1",
					req.URL.String())

				responseData, err = json.Marshal(apitype.ListPackagesResponse{
					Packages:          secondPagePackages,
					ContinuationToken: ptr("next-page-token-2"),
				})
				require.NoError(t, err)
			case 2:
				assert.Equal(t,
					"/api/registry/packages?limit=499&name=my-package&continuationToken=next-page-token-2",
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
		//nolint:prealloc // capacity unknown ahead of time
		searchResults := []apitype.PackageMetadata{}
		for pkg, err := range mockClient.ListPackages(t.Context(), &searchName) {
			require.NoError(t, err)
			searchResults = append(searchResults, pkg)
		}

		expectedPackages := slices.Concat(firstPagePackages, secondPagePackages, thirdPagePackages)
		assert.Equal(t, expectedPackages, searchResults)
		assert.Equal(t, 3, requestCount) // Ensure both requests were made
	})
}

func TestCallCopilot(t *testing.T) {
	t.Parallel()

	t.Run("StatusNoContent", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns 204 No Content, it should return an empty string without error
		noContentServer := newMockServer(http.StatusNoContent, "")
		defer noContentServer.Close()

		client := newMockClient(noContentServer)
		response, err := client.callCopilot(t.Context(), map[string]string{"test": "data"})

		require.NoError(t, err)
		require.Empty(t, response)
	})

	t.Run("StatusPaymentRequired", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns 402 Payment Required (usage limit), it should return an error with the body message
		usageLimitServer := newMockServer(http.StatusPaymentRequired, "Usage limit reached")
		defer usageLimitServer.Close()

		client := newMockClient(usageLimitServer)
		response, err := client.callCopilot(t.Context(), map[string]string{"test": "data"})

		require.EqualError(t, err, "Usage limit reached")
		require.Empty(t, response)
	})

	t.Run("OtherErrorStatus", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns other error status codes, it should return an error with the body message
		errorServer := newMockServer(http.StatusInternalServerError, "Internal server error")
		defer errorServer.Close()

		client := newMockClient(errorServer)
		response, err := client.callCopilot(t.Context(), map[string]string{"test": "data"})

		require.EqualError(t, err, "Internal server error")
		require.Empty(t, response)
	})

	t.Run("EmptyErrorBody", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns error status with empty body, it should return a generic error
		emptyBodyServer := newMockServer(http.StatusBadRequest, "")
		defer emptyBodyServer.Close()

		client := newMockClient(emptyBodyServer)
		response, err := client.callCopilot(t.Context(), map[string]string{"test": "data"})

		require.ErrorContains(t, err, "Copilot API returned error status: 400")
		require.Empty(t, response)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()

		// When Copilot API returns a non-JSON response with status 200, it should return an error
		invalidJSONServer := newMockServer(http.StatusOK, "This is not JSON")
		defer invalidJSONServer.Close()

		client := newMockClient(invalidJSONServer)
		response, err := client.callCopilot(t.Context(), map[string]string{"test": "data"})

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
		response, err := client.callCopilot(t.Context(), map[string]string{"test": "data"})

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
		response, err := client.callCopilot(t.Context(), map[string]string{"test": "data"})

		require.EqualError(t, err, "copilot API error: API error message\nDetailed error information")
		require.Empty(t, response)
	})
}

func TestCreateNeoTask(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		successServer := newMockServer(http.StatusCreated, `{"taskId": "task_abc123"}`)
		defer successServer.Close()

		client := newMockClient(successServer)
		resp, err := client.CreateNeoTask(
			t.Context(), "my-org", "Help me debug this error", "my-stack", "my-project", CreateNeoTaskOptions{})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "task_abc123", resp.TaskID)
	})

	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		errorServer := newMockServer(http.StatusBadRequest, `{"message": "Bad request"}`)
		defer errorServer.Close()

		client := newMockClient(errorServer)
		resp, err := client.CreateNeoTask(
			t.Context(), "my-org", "Help me debug this error", "my-stack", "my-project", CreateNeoTaskOptions{})

		require.Error(t, err)
		require.Nil(t, resp)
		assert.Contains(t, err.Error(), "creating Neo task")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()

		unauthorizedServer := newMockServer(http.StatusUnauthorized, "401: Unauthorized")
		defer unauthorizedServer.Close()

		client := newMockClient(unauthorizedServer)
		resp, err := client.CreateNeoTask(
			t.Context(), "my-org", "Help me debug this error", "my-stack", "my-project", CreateNeoTaskOptions{})

		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("RequestShape", func(t *testing.T) {
		t.Parallel()

		// The backend deserializes into apitype.CreateAgentTaskRequest, so the path,
		// toolExecutionMode camelCase tag, and entity_diff block have to land exactly
		// right. Anchor that wire shape here so a refactor can't silently drift it.
		var (
			gotPath   string
			gotMethod string
			gotBody   map[string]any
		)
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			gotPath = req.URL.Path
			gotMethod = req.Method
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_1"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hello", "stack", "proj", CreateNeoTaskOptions{
			ToolExecutionMode: "cli",
		})
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, gotMethod)
		assert.Equal(t, "/api/preview/agents/my-org/tasks", gotPath)
		assert.Equal(t, "cli", gotBody["toolExecutionMode"])
		// source must be "cli" on every task created via the CLI so the server can
		// attribute the task to its origin (matches apitype.AgentTaskSourceCli).
		assert.Equal(t, "cli", gotBody["source"], "CLI-originated tasks must send source:cli")
		// approvalMode is omitempty — must not appear in the body when empty so the
		// server falls back to its default (auto) mode.
		assert.NotContains(t, gotBody, "approvalMode", "empty approvalMode must be omitted")

		message, _ := gotBody["message"].(map[string]any)
		require.NotNil(t, message)
		assert.Equal(t, "user_message", message["type"])
		assert.Equal(t, "hello", message["content"])

		entityDiff, _ := message["entity_diff"].(map[string]any)
		require.NotNil(t, entityDiff, "stack+project must produce an entity_diff block")
		add, _ := entityDiff["add"].([]any)
		require.Len(t, add, 1)
		entity, _ := add[0].(map[string]any)
		assert.Equal(t, "stack", entity["type"])
		assert.Equal(t, "stack", entity["name"])
		assert.Equal(t, "proj", entity["project"])
	})

	t.Run("ApprovalModeManualSerializes", func(t *testing.T) {
		t.Parallel()

		// When the CLI passes NeoApprovalModeManual, the body must carry
		// approvalMode:"manual" so the server gates each tool call on a
		// user_approval_request. The wire tag is camelCase to match the IDL.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_3"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "stack", "proj", CreateNeoTaskOptions{
			ToolExecutionMode: "cli",
			ApprovalMode:      NeoApprovalModeManual,
		})
		require.NoError(t, err)

		assert.Equal(t, "manual", gotBody["approvalMode"])
	})

	t.Run("OmitsEntityDiffWhenStackMissing", func(t *testing.T) {
		t.Parallel()

		// The backend rejects entity_diff entries with empty name/project, so the
		// client must omit the block entirely when either side is missing.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_2"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "", "proj", CreateNeoTaskOptions{})
		require.NoError(t, err)

		message, _ := gotBody["message"].(map[string]any)
		require.NotNil(t, message)
		assert.NotContains(t, message, "entity_diff")
	})

	t.Run("PlanModeInRequestBody", func(t *testing.T) {
		t.Parallel()

		// planMode is a per-task feature flag plumbed on CreateAgentTaskRequest; the
		// JSON tag must be camelCase planMode, and an omitted (false) value must not
		// leak into the request so unrelated callers aren't implicitly opted in.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_3"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "plan this", "", "proj", CreateNeoTaskOptions{
			ToolExecutionMode: "cli",
			PlanMode:          true,
		})
		require.NoError(t, err)

		assert.Equal(t, true, gotBody["planMode"], "planMode=true must be sent as planMode:true")
	})

	t.Run("PlanModeOmittedWhenFalse", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_4"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "", "proj", CreateNeoTaskOptions{})
		require.NoError(t, err)

		assert.NotContains(t, gotBody, "planMode", "planMode must be omitted when false")
	})

	t.Run("PermissionModeReadOnlySerializes", func(t *testing.T) {
		t.Parallel()

		// permissionMode is per-task and the cloud caps OBO token scopes when it
		// reads "read-only". The wire tag is camelCase to match apitype.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_5"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "stack", "proj", CreateNeoTaskOptions{
			ToolExecutionMode: "cli",
			PermissionMode:    NeoPermissionModeReadOnly,
		})
		require.NoError(t, err)

		assert.Equal(t, "read-only", gotBody["permissionMode"])
	})

	t.Run("PermissionModeOmittedWhenEmpty", func(t *testing.T) {
		t.Parallel()

		// An empty PermissionMode must not appear in the body so the server
		// falls back to the org default rather than seeing an invalid value.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_6"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "", "proj", CreateNeoTaskOptions{})
		require.NoError(t, err)

		assert.NotContains(t, gotBody, "permissionMode")
	})

	t.Run("EnabledIntegrationsEmptyListSerializes", func(t *testing.T) {
		t.Parallel()

		// The --disable-integrations opt-out must reach the wire as an explicit empty
		// array, distinct from omitting the field entirely.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_7"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "stack", "proj", CreateNeoTaskOptions{
			ToolExecutionMode:   "cli",
			EnabledIntegrations: &[]string{},
		})
		require.NoError(t, err)

		require.Contains(t, gotBody, "enabledIntegrations",
			"an explicit opt-out must send enabledIntegrations, not omit it")
		assert.Equal(t, []any{}, gotBody["enabledIntegrations"],
			"enabledIntegrations must serialize as an empty JSON array")
	})

	t.Run("EnabledIntegrationsOmittedWhenNil", func(t *testing.T) {
		t.Parallel()

		// A nil pointer must omit the field so the server inherits the org default.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_8"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "", "proj", CreateNeoTaskOptions{})
		require.NoError(t, err)

		assert.NotContains(t, gotBody, "enabledIntegrations",
			"a nil EnabledIntegrations must be omitted so the server inherits the org default")
	})

	t.Run("EnabledIntegrationsAllowlistSerializes", func(t *testing.T) {
		t.Parallel()

		// A populated slice is the (future) allow-list state: named integrations
		// must carry through verbatim.
		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusCreated)
			_, _ = rw.Write([]byte(`{"taskId":"t_9"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.CreateNeoTask(t.Context(), "my-org", "hi", "stack", "proj", CreateNeoTaskOptions{
			ToolExecutionMode:   "cli",
			EnabledIntegrations: &[]string{"honeycomb", "datadog"},
		})
		require.NoError(t, err)

		assert.Equal(t, []any{"honeycomb", "datadog"}, gotBody["enabledIntegrations"])
	})
}

func TestUpdateNeoTask(t *testing.T) {
	t.Parallel()

	// UpdateNeoTask is the CLI's mid-session toggle path: it PATCHes /tasks/{id}
	// with whichever mode fields the user just toggled. The pointer fields on
	// UpdateNeoTaskOptions ensure that a single-axis toggle doesn't reset the
	// other axis on the server side.

	t.Run("ApprovalModeUpdate", func(t *testing.T) {
		t.Parallel()

		var (
			gotPath   string
			gotMethod string
			gotBody   map[string]any
		)
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			gotPath = req.URL.Path
			gotMethod = req.Method
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newMockClient(server)
		mode := NeoApprovalModeBalanced
		err := c.UpdateNeoTask(t.Context(), "my-org", "task_1", UpdateNeoTaskOptions{
			ApprovalMode: &mode,
		})
		require.NoError(t, err)

		assert.Equal(t, http.MethodPatch, gotMethod)
		assert.Equal(t, "/api/preview/agents/my-org/tasks/task_1", gotPath)
		assert.Equal(t, "balanced", gotBody["approvalMode"])
		assert.NotContains(t, gotBody, "permissionMode",
			"a single-axis toggle must not send the untouched axis")
	})

	t.Run("PermissionModeUpdate", func(t *testing.T) {
		t.Parallel()

		var gotBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
			rw.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newMockClient(server)
		perm := NeoPermissionModeReadOnly
		err := c.UpdateNeoTask(t.Context(), "my-org", "task_2", UpdateNeoTaskOptions{
			PermissionMode: &perm,
		})
		require.NoError(t, err)

		assert.Equal(t, "read-only", gotBody["permissionMode"])
		assert.NotContains(t, gotBody, "approvalMode")
	})

	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		errorServer := newMockServer(http.StatusBadRequest, `{"message": "bad"}`)
		defer errorServer.Close()

		c := newMockClient(errorServer)
		mode := NeoApprovalModeAuto
		err := c.UpdateNeoTask(t.Context(), "my-org", "task_x", UpdateNeoTaskOptions{
			ApprovalMode: &mode,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "updating Neo task")
	})
}

func TestPostNeoTaskUserEvent(t *testing.T) {
	t.Parallel()

	// The CLI loop must POST user events to the task root (not /events, which is
	// reserved for the agent runtime) and wrap them in the {"event": ...} envelope.
	var (
		gotPath   string
		gotMethod string
		gotBody   map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		require.NoError(t, json.NewDecoder(req.Body).Decode(&gotBody))
		rw.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newMockClient(server)
	err := client.PostNeoTaskUserEvent(t.Context(), "my-org", "task_1", apitype.AgentUserEventExecToolCall{
		Type:       "exec_tool_call",
		ToolCallID: "c1",
		Name:       "filesystem__read",
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/preview/agents/my-org/tasks/task_1", gotPath)

	event, _ := gotBody["event"].(map[string]any)
	require.NotNil(t, event, "body must wrap the inner event in {\"event\": ...}")
	assert.Equal(t, "exec_tool_call", event["type"])
	assert.Equal(t, "c1", event["tool_call_id"])
	assert.Equal(t, "filesystem__read", event["name"])
}

func TestStreamNeoTaskEvents(t *testing.T) {
	t.Parallel()

	t.Run("ParsesDataFramesAndIgnoresComments", func(t *testing.T) {
		t.Parallel()

		// SSE framing: blank lines delimit events, lines that start with ":" are
		// comments (heartbeats), and event-less `data:` frames concatenate with a
		// single newline separator. Exercise each in one stream.
		var gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			gotPath = req.URL.Path
			assert.Equal(t, "text/event-stream", req.Header.Get("Accept"))
			rw.Header().Set("Content-Type", "text/event-stream")
			rw.WriteHeader(http.StatusOK)
			flusher, ok := rw.(http.Flusher)
			require.True(t, ok)
			_, _ = rw.Write([]byte(": heartbeat\n"))
			_, _ = rw.Write([]byte("data: {\"type\":\"agentResponse\"}\n\n"))
			_, _ = rw.Write([]byte("data: line1\ndata: line2\n\n"))
			flusher.Flush()
		}))
		defer server.Close()

		client := newMockClient(server)
		stream, err := client.StreamNeoTaskEvents(t.Context(), "my-org", "task_1", "")
		require.NoError(t, err)

		got := make([][]byte, 0, 2)
		for evt := range stream {
			require.NoError(t, evt.Err)
			got = append(got, evt.Data)
		}
		assert.Equal(t, "/api/preview/agents/my-org/tasks/task_1/events/stream", gotPath)
		require.Len(t, got, 2)
		assert.Equal(t, `{"type":"agentResponse"}`, string(got[0]))
		assert.Equal(t, "line1\nline2", string(got[1]))
	})

	t.Run("HTTPErrorSurfacesBeforeStreamStarts", func(t *testing.T) {
		t.Parallel()

		// A non-2xx response must fail the initial handshake, not surface as a
		// stream error later — the caller should not have to drain a dead channel.
		server := newMockServer(http.StatusUnauthorized, "unauthorized")
		defer server.Close()

		client := newMockClient(server)
		stream, err := client.StreamNeoTaskEvents(t.Context(), "my-org", "task_1", "")
		require.Error(t, err)
		assert.Nil(t, stream)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("ParsesEventIDsAndSendsLastEventIDHeader", func(t *testing.T) {
		t.Parallel()

		// The Neo CLI tracks the last event ID it consumed and sends it back as
		// `Last-Event-ID` on reconnect so the service can replay missed events.
		// Verify both halves: outgoing header propagation and incoming `id:` parsing.
		var gotLastEventID string
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			gotLastEventID = req.Header.Get("Last-Event-ID")
			rw.Header().Set("Content-Type", "text/event-stream")
			rw.WriteHeader(http.StatusOK)
			flusher, ok := rw.(http.Flusher)
			require.True(t, ok)
			// First event carries id:42; second event omits id: — per the SSE spec
			// the "last event ID buffer" persists, so the second event also reports 42.
			_, _ = rw.Write([]byte("id: 42\ndata: a\n\n"))
			_, _ = rw.Write([]byte("data: b\n\n"))
			flusher.Flush()
		}))
		defer server.Close()

		client := newMockClient(server)
		stream, err := client.StreamNeoTaskEvents(t.Context(), "my-org", "task_1", "17")
		require.NoError(t, err)

		got := make([]NeoStreamEvent, 0, 2)
		for evt := range stream {
			require.NoError(t, evt.Err)
			got = append(got, evt)
		}
		assert.Equal(t, "17", gotLastEventID, "outgoing Last-Event-ID header")
		require.Len(t, got, 2)
		assert.Equal(t, "a", string(got[0].Data))
		assert.Equal(t, "42", got[0].ID)
		assert.Equal(t, "b", string(got[1].Data))
		assert.Equal(t, "42", got[1].ID, "id buffer should persist across events without id:")
	})

	t.Run("OmitsLastEventIDHeaderWhenEmpty", func(t *testing.T) {
		t.Parallel()

		// On the very first connection the caller has no event ID yet — make sure
		// we don't send `Last-Event-ID:` (with an empty value), which some proxies
		// reject.
		var headerWasSet bool
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			_, headerWasSet = req.Header["Last-Event-Id"]
			rw.Header().Set("Content-Type", "text/event-stream")
			rw.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newMockClient(server)
		stream, err := client.StreamNeoTaskEvents(t.Context(), "my-org", "task_1", "")
		require.NoError(t, err)
		for range stream {
		}
		assert.False(t, headerWasSet, "Last-Event-ID must not be sent when empty")
	})

	t.Run("ContextCancelClosesStream", func(t *testing.T) {
		t.Parallel()

		// Lifetime is caller-controlled via ctx — cancelling mid-stream must close
		// the channel promptly, even while the server is still holding the HTTP
		// connection open.
		ready := make(chan struct{})
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.Header().Set("Content-Type", "text/event-stream")
			rw.WriteHeader(http.StatusOK)
			flusher, _ := rw.(http.Flusher)
			if flusher != nil {
				flusher.Flush()
			}
			close(ready)
			<-release
		}))
		defer server.Close()
		defer close(release)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		client := newMockClient(server)
		stream, err := client.StreamNeoTaskEvents(ctx, "my-org", "task_1", "")
		require.NoError(t, err)

		<-ready
		cancel()

		for range stream {
			// Drain; just verifying the channel closes.
		}
	})
}

func TestRefreshAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("returns the new access token on success", func(t *testing.T) {
		t.Parallel()
		var gotPath, gotMethod, gotContentType, gotBody string
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			gotPath = req.URL.Path
			gotMethod = req.Method
			gotContentType = req.Header.Get("Content-Type")
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			gotBody = string(body)

			err = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken:     "new-obo-access-token",
				IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
				TokenType:       "Bearer",
				ExpiresIn:       3600,
				RefreshToken:    "rt-value", // server echoes the same refresh token (no rotation)
			})
			require.NoError(t, err)
		}))
		defer server.Close()

		resp, err := NewClient(server.URL, "", true, nil).RefreshAccessToken(t.Context(), "rt-value")
		require.NoError(t, err)
		require.NotNil(t, resp)

		assert.Equal(t, http.MethodPost, gotMethod)
		assert.Equal(t, "/api/oauth/token", gotPath)
		assert.Equal(t, "application/x-www-form-urlencoded", gotContentType)
		assert.Contains(t, gotBody, "grant_type=refresh_token")
		assert.Contains(t, gotBody, "refresh_token=rt-value")
		assert.Equal(t, "new-obo-access-token", resp.AccessToken)
		assert.Equal(t, "Bearer", resp.TokenType)
		assert.Equal(t, int64(3600), resp.ExpiresIn)
		assert.Equal(t, "rt-value", resp.RefreshToken)
	})

	t.Run("rejects an empty refresh token without hitting the server", func(t *testing.T) {
		t.Parallel()
		// Server fails the test if called — the empty-string check must short-circuit.
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			t.Errorf("server should not have been called for empty refresh token")
		}))
		defer server.Close()

		_, err := NewClient(server.URL, "", true, nil).RefreshAccessToken(t.Context(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refresh token is required")
	})

	t.Run("surfaces server-side invalid_grant errors verbatim", func(t *testing.T) {
		t.Parallel()
		// Mirrors what the service returns when the row is gone / wrong-type / soft-deleted (see
		// cmd/service/api/oauth2/grant_type_refresh_token.go in the pulumi-service repo).
		const errBody = `{"error":"invalid_grant","error_description":"refresh token is not valid"}`
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusBadRequest)
			_, err := rw.Write([]byte(errBody))
			require.NoError(t, err)
		}))
		defer server.Close()

		_, err := NewClient(server.URL, "", true, nil).RefreshAccessToken(t.Context(), "rt-revoked")
		require.Error(t, err)
		// Caller must see both the status and the original error_description so they can branch
		// on invalid_grant (give up, prompt for login) vs unsupported_grant_type (LD kill switch,
		// retry later).
		assert.Contains(t, err.Error(), "400")
		assert.Contains(t, err.Error(), "invalid_grant")
		assert.Contains(t, err.Error(), "refresh token is not valid")
	})

	t.Run("rejects an empty access_token in the response", func(t *testing.T) {
		t.Parallel()
		// Defensive: a malformed 200 with no access_token must not be silently treated as success.
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			_, err := rw.Write([]byte(`{"access_token":"","token_type":"Bearer"}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		_, err := NewClient(server.URL, "", true, nil).RefreshAccessToken(t.Context(), "rt-value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty access_token")
	})
}

func TestRefreshableAPIAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("Refresh replaces the access token and persists via writeback", func(t *testing.T) {
		t.Parallel()

		var seenRefreshToken, seenWriteAT, seenWriteRT string
		tok := &refreshableAPIAccessToken{
			accessToken:  "stale-access",
			refreshToken: "stale-refresh",
			refresh: func(_ context.Context, rt string) (string, time.Time, string, error) {
				seenRefreshToken = rt
				return "new-access", time.Time{}, "new-refresh", nil
			},
			writeback: func(at string, _ time.Time, rt string) error {
				seenWriteAT, seenWriteRT = at, rt
				return nil
			},
		}

		require.NoError(t, tok.Refresh(t.Context(), "stale-access"))

		got, err := tok.Get(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "new-access", got)
		assert.Equal(t, "stale-refresh", seenRefreshToken)
		assert.Equal(t, "new-access", seenWriteAT)
		assert.Equal(t, "new-refresh", seenWriteRT)
	})

	t.Run("Refresh keeps the existing refresh token when the server returns empty (no rotation)", func(t *testing.T) {
		t.Parallel()

		tok := &refreshableAPIAccessToken{
			accessToken:  "stale-access",
			refreshToken: "stable-refresh",
			refresh: func(_ context.Context, _ string) (string, time.Time, string, error) {
				return "new-access", time.Time{}, "", nil
			},
			writeback: func(at string, _ time.Time, rt string) error { return nil },
		}
		require.NoError(t, tok.Refresh(t.Context(), "stale-access"))

		var seenSecondRefreshToken string
		tok.refresh = func(_ context.Context, rt string) (string, time.Time, string, error) {
			seenSecondRefreshToken = rt
			return "newer-access", time.Time{}, "", nil
		}
		require.NoError(t, tok.Refresh(t.Context(), "new-access"))
		assert.Equal(t, "stable-refresh", seenSecondRefreshToken,
			"the wrapper continues to send the original refresh token across calls")
	})

	t.Run("Refresh failure surfaces the underlying error and leaves state untouched", func(t *testing.T) {
		t.Parallel()

		var writebackCalled bool
		tok := &refreshableAPIAccessToken{
			accessToken:  "original-access",
			refreshToken: "original-refresh",
			refresh: func(_ context.Context, _ string) (string, time.Time, string, error) {
				return "", time.Time{}, "", errors.New("invalid_grant")
			},
			writeback: func(at string, _ time.Time, rt string) error {
				writebackCalled = true
				return nil
			},
		}

		err := tok.Refresh(t.Context(), "original-access")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_grant")
		assert.False(t, writebackCalled, "writeback must not fire when the refresh attempt fails")

		got, gerr := tok.Get(t.Context())
		require.NoError(t, gerr)
		assert.Equal(t, "original-access", got, "Get returns the original token when the refresh failed")
	})

	t.Run("Refresh surfaces writeback failure", func(t *testing.T) {
		t.Parallel()

		tok := &refreshableAPIAccessToken{
			accessToken:  "original-access",
			refreshToken: "original-refresh",
			refresh: func(_ context.Context, _ string) (string, time.Time, string, error) {
				return "new-access", time.Time{}, "new-refresh", nil
			},
			writeback: func(_ string, _ time.Time, _ string) error { return errors.New("disk full") },
		}

		err := tok.Refresh(t.Context(), "original-access")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "disk full")
	})
}

func TestClient_WithRefresh(t *testing.T) {
	t.Parallel()

	t.Run("end-to-end: stale token triggers refresh and retry, writeback fires", func(t *testing.T) {
		t.Parallel()

		var apiCalls, refreshCalls atomic.Int32
		var seenAuths []string
		var mu sync.Mutex

		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			switch req.URL.Path {
			case "/api/oauth/token":
				refreshCalls.Add(1)
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Contains(t, string(body), "grant_type=refresh_token")
				assert.Contains(t, string(body), "refresh_token=stale-refresh")
				_ = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
					AccessToken:  "fresh-access",
					TokenType:    "Bearer",
					ExpiresIn:    3600,
					RefreshToken: "stale-refresh",
				})
			case "/api/user":
				mu.Lock()
				seenAuths = append(seenAuths, req.Header.Get("Authorization"))
				mu.Unlock()
				n := apiCalls.Add(1)
				if n == 1 {
					rw.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
					return
				}
				_ = json.NewEncoder(rw).Encode(serviceUser{GitHubLogin: "alice"})
			default:
				rw.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		var writeAT, writeRT string
		var writeExpiresAt time.Time
		pc := NewClient(srv.URL, "stale-access", true, nil)
		pc.WithRefresh("stale-refresh", func(at string, expiresAt time.Time, rt string) error {
			writeAT, writeRT, writeExpiresAt = at, rt, expiresAt
			return nil
		})

		name, _, _, err := pc.GetPulumiAccountDetails(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "alice", name)
		assert.Equal(t, int32(2), apiCalls.Load(), "/api/user should be tried twice")
		assert.Equal(t, int32(1), refreshCalls.Load())
		require.Len(t, seenAuths, 2)
		assert.Equal(t, "token stale-access", seenAuths[0])
		assert.Equal(t, "token fresh-access", seenAuths[1])
		assert.Equal(t, "fresh-access", writeAT, "writeback receives the refreshed access token")
		assert.Equal(t, "stale-refresh", writeRT,
			"writeback receives the still-current refresh token (no rotation in Phase 1)")
		assert.False(t, writeExpiresAt.IsZero(),
			"writeback receives the new access token's ExpiresAt derived from the grant's ExpiresIn")
		assert.True(t, writeExpiresAt.After(time.Now().Add(50*time.Minute)),
			"the new ExpiresAt is roughly now+ExpiresIn (3600s in this fixture)")
	})

	t.Run("empty refresh token leaves the plain access token in place", func(t *testing.T) {
		t.Parallel()

		pc := NewClient("https://api.example.com", "tok", false, nil)
		pc.WithRefresh("", func(at string, _ time.Time, rt string) error { return nil })
		_, isRefreshable := pc.apiToken.(refreshable)
		assert.False(t, isRefreshable, "an empty refresh token must not swap in a refreshable wrapper")
	})

	t.Run("nil writeback with non-empty refresh token fails the precondition", func(t *testing.T) {
		t.Parallel()

		// A non-empty refresh token without a writeback is a programmer error — a refresh would
		// crash at call time. Catch it at the wiring site, where the violated precondition can
		// be named, rather than waiting for the first 401.
		pc := NewClient("https://api.example.com", "tok", false, nil)
		assert.Panics(t, func() { pc.WithRefresh("some-refresh", nil) })
	})
}

func TestPublishPolicyPackPerPlatform(t *testing.T) {
	t.Parallel()

	uploads := map[string][]byte{}
	var createReq apitype.CreatePolicyPackRequest
	completeCalled := false

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/orgs/acme/policypacks", func(rw http.ResponseWriter, req *http.Request) {
		require.NoError(t, json.NewDecoder(req.Body).Decode(&createReq))
		resp := apitype.CreatePolicyPackResponse{
			Version: 1,
			PlatformUploadURIs: map[string]apitype.PolicyPackUpload{
				"linux-amd64": {
					UploadURI:       server.URL + "/upload/linux-amd64",
					RequiredHeaders: map[string]string{"x-test": "yes"},
				},
				"darwin-arm64": {UploadURI: server.URL + "/upload/darwin-arm64"},
			},
		}
		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/upload/", func(rw http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodPut, req.Method)
		platform := strings.TrimPrefix(req.URL.Path, "/upload/")
		if platform == "linux-amd64" {
			require.Equal(t, "yes", req.Header.Get("x-test"))
		}
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		uploads[platform] = body
	})
	mux.HandleFunc("/api/orgs/acme/policypacks/mypack/versions/0.0.1/complete",
		func(rw http.ResponseWriter, req *http.Request) {
			completeCalled = true
		})

	client := newMockClient(server)
	version, err := client.PublishPolicyPack(t.Context(), "acme", "executable",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		PolicyPackArtifacts{PerPlatform: map[string][]byte{
			"linux-amd64":  []byte("linux-bytes"),
			"darwin-arm64": []byte("darwin-bytes"),
		}}, nil)
	require.NoError(t, err)

	assert.Equal(t, "0.0.1", version)
	assert.Equal(t, []string{"darwin-arm64", "linux-amd64"}, createReq.Platforms)
	assert.Equal(t, "executable", createReq.Runtime)
	assert.Equal(t, []byte("linux-bytes"), uploads["linux-amd64"])
	assert.Equal(t, []byte("darwin-bytes"), uploads["darwin-arm64"])
	assert.True(t, completeCalled)
}

func TestPublishPolicyPackPerPlatformUploadRejected(t *testing.T) {
	t.Parallel()

	completeCalled := false

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/orgs/acme/policypacks", func(rw http.ResponseWriter, req *http.Request) {
		resp := apitype.CreatePolicyPackResponse{
			Version: 1,
			PlatformUploadURIs: map[string]apitype.PolicyPackUpload{
				"darwin-arm64": {UploadURI: server.URL + "/upload/darwin-arm64"},
				"linux-amd64":  {UploadURI: server.URL + "/upload/linux-amd64"},
			},
		}
		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/upload/", func(rw http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodPut, req.Method)
		platform := strings.TrimPrefix(req.URL.Path, "/upload/")
		if platform == "darwin-arm64" {
			rw.WriteHeader(http.StatusForbidden)
		}
	})
	mux.HandleFunc("/api/orgs/acme/policypacks/mypack/versions/0.0.1/complete",
		func(rw http.ResponseWriter, req *http.Request) {
			completeCalled = true
		})

	client := newMockClient(server)
	_, err := client.PublishPolicyPack(t.Context(), "acme", "executable",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		PolicyPackArtifacts{PerPlatform: map[string][]byte{
			"linux-amd64":  []byte("linux-bytes"),
			"darwin-arm64": []byte("darwin-bytes"),
		}}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "darwin-arm64")
	assert.False(t, completeCalled)
}

func TestPublishPolicyPackPerPlatformUnsupportedService(t *testing.T) {
	t.Parallel()

	server := newMockServer(200, `{"version":1,"uploadURI":"https://single-artifact-only"}`)
	defer server.Close()

	client := newMockClient(server)
	_, err := client.PublishPolicyPack(t.Context(), "acme", "executable",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		PolicyPackArtifacts{PerPlatform: map[string][]byte{"linux-amd64": []byte("b")}}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "does not support per-platform policy pack artifacts")
}
