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
// with an optional label override. It answers two distinct questions in order:
//
//  1. Whose credentials ran this update? When they are the user's own, the
//     console permalink is reachable and is returned unchanged. Only actual
//     credential use counts here: a leftover claim file in the shared agent
//     store must not affect commands that ran on user credentials.
//  2. The update ran on ephemeral agent-account credentials, so the permalink
//     points at the agent's organization, which the user cannot open until the
//     account is claimed. Show the claim URL while the claim is active for
//     this backend, and otherwise show nothing at all — never the unreachable
//     permalink.
func permalinkForDisplay(ctx context.Context, cloudURL, permalink string) (string, string) {
	if !AgentCredentialsUsed(ctx, cloudURL) {
		return permalink, ""
	}
	if claim, err := workspace.GetAgentClaim(); err == nil &&
		claim.CloudURL == cloudURL && claim.Active(time.Now()) {
		return claim.ClaimURL, agentClaimPermalinkLabel
	}
	return "", ""
}
