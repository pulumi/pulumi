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

package agentauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatAgentClaimInstruction(t *testing.T) {
	t.Parallel()

	claimURL := "https://app.pulumi.com/claim/abc123"
	now := time.Date(2026, time.May, 17, 8, 24, 56, 0, time.UTC)
	validUntil := now.Add(3*24*time.Hour + 4*time.Hour + 10*time.Minute + 30*time.Second)
	expiresAt := now.Add(2*24*time.Hour + 3*time.Hour + 9*time.Minute)

	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"CLAIM_URL=https://app.pulumi.com/claim/abc123\n"+
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=2d3h9m\n"+
			"ACTION_REQUIRED=Tell the user to claim this Pulumi agent account.\n"+
			"INSTRUCTION=Tell the user Pulumi created an ephemeral agent account. "+
			"It is important to surface the claim URL to the user so they can take ownership of the account. "+
			"Include the claim URL and say they must claim the account within 2d3h9m before agent access expires. "+
			"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
			"so they should let the agent finish its work before claiming it.\n",
		formatAgentClaimInstruction(claimURL, &expiresAt, validUntil, now))
	expiredAt := now.Add(-time.Minute)
	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"CLAIM_URL=https://app.pulumi.com/claim/abc123\n"+
			"CLAIM_URL_VALID_FOR=3d4h10m\n"+
			"ACTION_REQUIRED=Tell the user to claim this Pulumi agent account.\n"+
			"INSTRUCTION=Tell the user this ephemeral agent account can no longer authenticate, "+
			"but the claim URL is still valid for 3d4h10m. Include the claim URL and the remaining time. "+
			"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
			"so they should let the agent finish its work before claiming it.\n",
		formatAgentClaimInstruction(claimURL, &expiredAt, validUntil, now))
	assert.Empty(t, formatAgentClaimInstruction(claimURL, nil, time.Time{}, now))
	assert.Empty(t, formatAgentClaimInstruction("", &expiresAt, validUntil, now))
}

func TestFormatAgentLoginRequiredInstruction(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 17, 8, 24, 56, 0, time.UTC)
	expiresAt := now.Add(2*24*time.Hour + 3*time.Hour + 9*time.Minute)

	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=2d3h9m\n"+
			"ACTION_REQUIRED=Tell the user to run pulumi login.\n"+
			"INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate "+
			"even though local access had not expired. The account was likely claimed or revoked. "+
			"The stacks the agent was working with may have moved to the user's Pulumi account, so the agent's "+
			"existing access to those stacks may have changed. Ask the user to run pulumi login before retrying.\n",
		formatAgentLoginRequiredInstruction(agentLoginTokenRejected, &expiresAt, now))
	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=2d3h9m\n"+
			"ACTION_REQUIRED=Tell the user to run pulumi login.\n"+
			"INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate, "+
			"and its claim URL is no longer claimable. The account was likely already claimed, expired, "+
			"or revoked. If it was claimed, the stacks the agent was working with moved to the user's Pulumi account, "+
			"so the agent's existing access to those stacks changed. Ask the user to run pulumi login before retrying.\n",
		formatAgentLoginRequiredInstruction(agentLoginClaimUnavailable, &expiresAt, now))
}

func TestFormatAgentClaimValidFor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 17, 8, 24, 56, 0, time.UTC)
	tests := []struct {
		name       string
		validUntil time.Time
		want       string
	}{
		{
			name:       "days hours minutes",
			validUntil: now.Add(3*24*time.Hour + 4*time.Hour + 10*time.Minute + 30*time.Second),
			want:       "3d4h10m",
		},
		{
			name:       "hours only",
			validUntil: now.Add(2 * time.Hour),
			want:       "2h",
		},
		{
			name:       "less than minute",
			validUntil: now.Add(30 * time.Second),
			want:       "<1m",
		},
		{
			name:       "expired",
			validUntil: now.Add(-time.Minute),
			want:       "expired",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatAgentClaimValidFor(tt.validUntil, now))
		})
	}
}
