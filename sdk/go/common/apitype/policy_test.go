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

package apitype

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyPackPlatformFieldsRoundTrip(t *testing.T) {
	t.Parallel()

	req := CreatePolicyPackRequest{Name: "p", Platforms: []string{"linux-amd64"}}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"platforms":["linux-amd64"]`)

	var resp CreatePolicyPackResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"version": 2,
		"uploadURI": "https://legacy",
		"platformUploadURIs": {
			"linux-amd64": {"uploadURI": "https://x", "requiredHeaders": {"a": "b"}}
		}
	}`), &resp))
	assert.Equal(t, "https://legacy", resp.UploadURI)
	assert.Equal(t, "https://x", resp.PlatformUploadURIs["linux-amd64"].UploadURI)
	assert.Equal(t, map[string]string{"a": "b"}, resp.PlatformUploadURIs["linux-amd64"].RequiredHeaders)

	var rp RequiredPolicy
	require.NoError(t, json.Unmarshal([]byte(`{
		"name": "p",
		"packLocation": "legacy-key"
	}`), &rp))
	assert.Equal(t, "legacy-key", rp.PackLocation)
}

func TestPolicyPackPlatformFieldsOmitted(t *testing.T) {
	t.Parallel()

	b, err := json.Marshal(CreatePolicyPackRequest{Name: "p"})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "platforms")

	b, err = json.Marshal(RequiredPolicy{Name: "p"})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "packLocation")
}
