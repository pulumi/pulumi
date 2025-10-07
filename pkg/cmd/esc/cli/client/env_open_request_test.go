// Copyright 2025, Pulumi Corporation.

package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateEnvironmentOpenRequest(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		expectedResponse := &CreateEnvironmentOpenRequestResponse{
			ChangeRequests: []EnvironmentOpenRequestChangeRequest{
				{
					ProjectName:          "test-project",
					EnvironmentName:      "test-env",
					ChangeRequestID:      "24a7e5fd-dd10-400e-9286-19f66dd163fa",
					LatestRevisionNumber: 0,
					ETag:                 "24a7e5fd-dd10-400e-9286-19f66dd163fa/0",
				},
			},
		}

		testClient := newTestClient(t, http.MethodPost, "/api/esc/environments/test-org/test-project/test-env/open/request", func(w http.ResponseWriter, r *http.Request) {
			// Verify request body contains correct parameters
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var reqData struct {
				GrantExpirationSeconds int `json:"grantExpirationSeconds"`
				AccessDurationSeconds  int `json:"accessDurationSeconds"`
			}
			err = json.Unmarshal(body, &reqData)
			require.NoError(t, err)

			assert.Equal(t, 3600, reqData.GrantExpirationSeconds)
			assert.Equal(t, 7200, reqData.AccessDurationSeconds)

			// Return expected response
			err = json.NewEncoder(w).Encode(expectedResponse)
			require.NoError(t, err)
		})

		resp, err := testClient.CreateEnvironmentOpenRequest(
			context.Background(),
			"test-org",
			"test-project",
			"test-env",
			3600,
			7200,
		)

		require.NoError(t, err)
		assert.Equal(t, expectedResponse, resp)
	})

	t.Run("Default parameters", func(t *testing.T) {
		expectedResponse := &CreateEnvironmentOpenRequestResponse{
			ChangeRequests: []EnvironmentOpenRequestChangeRequest{
				{
					ProjectName:          "test-project",
					EnvironmentName:      "test-env",
					ChangeRequestID:      "24a7e5fd-dd10-400e-9286-19f66dd163fa",
					LatestRevisionNumber: 0,
					ETag:                 "24a7e5fd-dd10-400e-9286-19f66dd163fa/0",
				},
			},
		}

		testClient := newTestClient(t, http.MethodPost, "/api/esc/environments/test-org/test-project/test-env/open/request", func(w http.ResponseWriter, r *http.Request) {
			// Verify request body contains default parameters
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var reqData struct {
				GrantExpirationSeconds int `json:"grantExpirationSeconds"`
				AccessDurationSeconds  int `json:"accessDurationSeconds"`
			}
			err = json.Unmarshal(body, &reqData)
			require.NoError(t, err)

			assert.Equal(t, 90000, reqData.GrantExpirationSeconds)
			assert.Equal(t, 259200, reqData.AccessDurationSeconds)

			// Return expected response
			err = json.NewEncoder(w).Encode(expectedResponse)
			require.NoError(t, err)
		})

		resp, err := testClient.CreateEnvironmentOpenRequest(
			context.Background(),
			"test-org",
			"test-project",
			"test-env",
			90000,
			259200,
		)

		require.NoError(t, err)
		assert.Equal(t, expectedResponse, resp)
	})

	t.Run("Not Found", func(t *testing.T) {
		testClient := newTestClient(t, http.MethodPost, "/api/esc/environments/test-org/test-project/test-env/open/request", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    404,
				Message: "environment not found",
			})
			require.NoError(t, err)
		})

		resp, err := testClient.CreateEnvironmentOpenRequest(
			context.Background(),
			"test-org",
			"test-project",
			"test-env",
			3600,
			7200,
		)

		assert.Nil(t, resp)
		assert.ErrorContains(t, err, "environment not found")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		testClient := newTestClient(t, http.MethodPost, "/api/esc/environments/test-org/test-project/test-env/open/request", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    401,
				Message: "unauthorized",
			})
			require.NoError(t, err)
		})

		resp, err := testClient.CreateEnvironmentOpenRequest(
			context.Background(),
			"test-org",
			"test-project",
			"test-env",
			3600,
			7200,
		)

		assert.Nil(t, resp)
		assert.ErrorContains(t, err, "unauthorized")
	})

	t.Run("Forbidden", func(t *testing.T) {
		testClient := newTestClient(t, http.MethodPost, "/api/esc/environments/test-org/test-project/test-env/open/request", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    403,
				Message: "insufficient permissions to create open request",
			})
			require.NoError(t, err)
		})

		resp, err := testClient.CreateEnvironmentOpenRequest(
			context.Background(),
			"test-org",
			"test-project",
			"test-env",
			3600,
			7200,
		)

		assert.Nil(t, resp)
		assert.ErrorContains(t, err, "insufficient permissions to create open request")
	})
}
