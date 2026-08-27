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

func TestPermalinkForDisplayWithoutAgentCredentials(t *testing.T) {
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	// A leftover claim in the shared agent store must not affect commands
	// that ran on user credentials.
	require.NoError(t, workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   testClaimURL,
		CloudURL:   testCloudURL,
		ValidUntil: time.Now().Add(24 * time.Hour),
	}))

	ctx := ContextWithAgentCredentialUse(t.Context())
	permalink, label := permalinkForDisplay(ctx, testCloudURL, testViewLiveLink)
	assert.Equal(t, testViewLiveLink, permalink)
	assert.Empty(t, label)
}

func TestPermalinkForDisplayWithAgentCredentials(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name  string
		claim *workspace.AgentClaim
		want  string
	}{
		{
			name: "active claim",
			claim: &workspace.AgentClaim{
				ClaimURL:   testClaimURL,
				CloudURL:   testCloudURL,
				ValidUntil: future,
			},
			want: testClaimURL,
		},
		{
			name: "no expiry recorded",
			claim: &workspace.AgentClaim{
				ClaimURL: testClaimURL,
				CloudURL: testCloudURL,
			},
			want: testClaimURL,
		},
		{
			name:  "no claim stored",
			claim: nil,
			want:  "",
		},
		{
			name: "different backend",
			claim: &workspace.AgentClaim{
				ClaimURL:   testClaimURL,
				CloudURL:   "https://api.other.example.com",
				ValidUntil: future,
			},
			want: "",
		},
		{
			name: "expired claim",
			claim: &workspace.AgentClaim{
				ClaimURL:   testClaimURL,
				CloudURL:   testCloudURL,
				ValidUntil: past,
			},
			want: "",
		},
		{
			name: "claim marked unavailable",
			claim: &workspace.AgentClaim{
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
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
			if tt.claim != nil {
				require.NoError(t, workspace.StoreAgentClaim(*tt.claim))
			}

			ctx := ContextWithAgentCredentialUse(t.Context())
			MarkAgentCredentialsUsed(ctx, testCloudURL)

			permalink, label := permalinkForDisplay(ctx, testCloudURL, testViewLiveLink)
			assert.Equal(t, tt.want, permalink)
			assert.Equal(t, agentClaimPermalinkLabel, label)
		})
	}
}
