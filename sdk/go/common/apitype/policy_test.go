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

	req := CreatePolicyPackRequest{Name: "p", Platforms: []string{"linux-amd64", "darwin-arm64"}}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"platforms":["linux-amd64","darwin-arm64"]`)

	respJSON := `{"version":3,"platformUploadURIs":{"linux-amd64":{"uploadURI":"https://u","requiredHeaders":{"k":"v"}}}}`
	var resp CreatePolicyPackResponse
	require.NoError(t, json.Unmarshal([]byte(respJSON), &resp))
	assert.Equal(t, "https://u", resp.PlatformUploadURIs["linux-amd64"].UploadURI)
	assert.Equal(t, map[string]string{"k": "v"}, resp.PlatformUploadURIs["linux-amd64"].RequiredHeaders)

	rpJSON := `{"name":"p","version":1,"versionTag":"0.0.1","displayName":"p","packLocations":{"linux-amd64":"https://d"}}`
	var rp RequiredPolicy
	require.NoError(t, json.Unmarshal([]byte(rpJSON), &rp))
	assert.Equal(t, map[string]string{"linux-amd64": "https://d"}, rp.PackLocations)

	legacy := CreatePolicyPackRequest{Name: "p"}
	b, err = json.Marshal(legacy)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "platforms")
}
