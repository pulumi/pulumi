// Copyright 2026, Pulumi Corporation.
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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInsightsResource(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed resource", func(t *testing.T) {
		t.Parallel()

		want := apitype.InsightsResourceWithVersion{
			Account:     "prod-aws",
			Type:        "aws:s3/bucket:Bucket",
			ID:          "my-bucket",
			Version:     7,
			Modified:    time.Date(2026, 5, 1, 14, 30, 0, 0, time.UTC),
			State:       json.RawMessage(`{"arn":"arn:aws:s3:::my-bucket"}`),
			PolicyState: "compliant",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		client := newMockClient(server)
		got, err := client.GetInsightsResource(t.Context(), "acme", "prod-aws", "aws:s3/bucket:Bucket::my-bucket")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("double-encodes accountName and resourceTypeAndId", func(t *testing.T) {
		t.Parallel()

		// The service double-decodes these path parameters, so the wire form
		// must be double-encoded. `/` becomes `%2F` once, then `%252F`. The
		// percent sign of the first encoding (`%`) is what becomes `%25` on
		// the second pass.
		var capturedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// r.URL.Path runs the request through one layer of decoding, so
			// we read RequestURI to see the raw bytes the client sent.
			capturedPath = r.URL.EscapedPath()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"account":"x","type":"y","id":"z","version":1,"modified":"2026-01-01T00:00:00Z"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsResource(t.Context(),
			"acme", "team/a", "aws:s3/bucket:Bucket::my-bucket")
		require.NoError(t, err)

		// orgName is single-encoded ("acme" has no special chars to encode);
		// accountName and resourceTypeAndId are double-encoded.
		assert.Equal(t,
			"/api/preview/insights/acme/accounts/team%252Fa/resources/aws:s3%252Fbucket:Bucket::my-bucket",
			capturedPath,
		)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsResource(t.Context(),
			"acme", "prod-aws", "aws:s3/bucket:Bucket::missing")
		require.Error(t, err)
	})
}
