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

package httpstate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	testCloudURL     = "https://api.pulumi.com"
	testClaimURL     = "https://app.pulumi.com/claim/claim-token"
	testViewLiveLink = "https://app.pulumi.com/org/proj/stack/updates/1"
)

func TestAgentClaimPermalink(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name  string
		claim workspace.AgentClaim
		want  string
	}{
		{
			name: "valid claim",
			claim: workspace.AgentClaim{
				ClaimURL:   testClaimURL,
				CloudURL:   testCloudURL,
				ValidUntil: future,
			},
			want: testClaimURL,
		},
		{
			name: "no expiry recorded",
			claim: workspace.AgentClaim{
				ClaimURL: testClaimURL,
				CloudURL: testCloudURL,
			},
			want: testClaimURL,
		},
		{
			name:  "no claim",
			claim: workspace.AgentClaim{},
			want:  "",
		},
		{
			name: "different backend",
			claim: workspace.AgentClaim{
				ClaimURL:   testClaimURL,
				CloudURL:   "https://api.other.example.com",
				ValidUntil: future,
			},
			want: "",
		},
		{
			name: "expired claim",
			claim: workspace.AgentClaim{
				ClaimURL:   testClaimURL,
				CloudURL:   testCloudURL,
				ValidUntil: past,
			},
			want: "",
		},
		{
			name: "claim marked unavailable",
			claim: workspace.AgentClaim{
				ClaimURL:           testClaimURL,
				CloudURL:           testCloudURL,
				ValidUntil:         future,
				ClaimUnavailableAt: &past,
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, agentClaimPermalink(tt.claim, testCloudURL, now))
		})
	}
}

func TestPermalinkForDisplayWithoutAgentCredentials(t *testing.T) {
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	ctx := ContextWithAgentCredentialUse(t.Context())
	permalink, label := permalinkForDisplay(ctx, testCloudURL, testViewLiveLink)
	assert.Equal(t, testViewLiveLink, permalink)
	assert.Empty(t, label)
}

func TestPermalinkForDisplayWithAgentCredentials(t *testing.T) {
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	require.NoError(t, workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   testClaimURL,
		CloudURL:   testCloudURL,
		ValidUntil: time.Now().Add(24 * time.Hour),
	}))

	ctx := ContextWithAgentCredentialUse(t.Context())
	MarkAgentCredentialsUsed(ctx, testCloudURL)

	permalink, label := permalinkForDisplay(ctx, testCloudURL, testViewLiveLink)
	assert.Equal(t, testClaimURL, permalink)
	assert.Equal(t, agentClaimPermalinkLabel, label)
}

func TestPermalinkForDisplayWithAgentCredentialsAndUnusableClaim(t *testing.T) {
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	unavailableAt := time.Now().Add(-time.Minute)
	require.NoError(t, workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:           testClaimURL,
		CloudURL:           testCloudURL,
		ValidUntil:         time.Now().Add(24 * time.Hour),
		ClaimUnavailableAt: &unavailableAt,
	}))

	ctx := ContextWithAgentCredentialUse(t.Context())
	MarkAgentCredentialsUsed(ctx, testCloudURL)

	permalink, _ := permalinkForDisplay(ctx, testCloudURL, testViewLiveLink)
	assert.Empty(t, permalink)
}
