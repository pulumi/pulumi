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
	"context"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// agentClaimPermalinkLabel replaces the default permalink label when the
// permalink is swapped for the agent account's claim URL.
const agentClaimPermalinkLabel = "Claim this account to view in Pulumi Cloud"

// permalinkForDisplay returns the permalink to display for an update together
// with an optional label override. When the CLI runs on shared ephemeral
// agent-account credentials, the console permalink points at the agent's
// organization, which the user cannot access until the account is claimed, so
// the account's claim URL is shown instead; when the claim is no longer usable
// the permalink is suppressed entirely.
func permalinkForDisplay(ctx context.Context, cloudURL, permalink string) (string, string) {
	if !AgentCredentialsUsed(ctx, cloudURL) {
		return permalink, ""
	}
	claim, err := workspace.GetAgentClaim()
	if err != nil {
		return "", ""
	}
	return agentClaimPermalink(claim, cloudURL, time.Now()), agentClaimPermalinkLabel
}

// agentClaimPermalink returns the claim URL to display in place of a console
// permalink, or "" when the claim cannot be surfaced: it belongs to a
// different backend, has expired, or is no longer claimable.
func agentClaimPermalink(claim workspace.AgentClaim, cloudURL string, now time.Time) string {
	if claim.ClaimURL == "" || claim.CloudURL != cloudURL {
		return ""
	}
	if claim.ClaimUnavailableAt != nil {
		return ""
	}
	if !claim.ValidUntil.IsZero() && !claim.ValidUntil.After(now) {
		return ""
	}
	return claim.ClaimURL
}
