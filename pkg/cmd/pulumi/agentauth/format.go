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
	"fmt"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// formatAgentClaimInstruction returns the claim instruction with the access-token expiry,
// or the claim URL expiry when the access token is no longer valid.
func formatAgentClaimInstruction(
	claimURL string,
	accessTokenExpiresAt *time.Time,
	claimValidUntil time.Time,
	now time.Time,
) string {
	if claimURL == "" {
		return ""
	}
	var expirationLine string
	var instruction string
	if accessTokenExpiresAt != nil && accessTokenExpiresAt.After(now) {
		validFor := formatAgentClaimValidFor(*accessTokenExpiresAt, now)
		expirationLine = fmt.Sprintf("EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=%s\n", validFor)
		instruction = fmt.Sprintf(
			"Tell the user Pulumi created an ephemeral agent account. "+
				"It is important to surface the claim URL to the user so they can take ownership of the account. "+
				"Include the claim URL and say they must claim the account within %s before agent access expires. "+
				"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
				"so they should let the agent finish its work before claiming it.\n",
			validFor)
	} else if !claimValidUntil.IsZero() && claimValidUntil.After(now) {
		validFor := formatAgentClaimValidFor(claimValidUntil, now)
		expirationLine = fmt.Sprintf("CLAIM_URL_VALID_FOR=%s\n", validFor)
		instruction = fmt.Sprintf(
			"Tell the user this ephemeral agent account can no longer authenticate, "+
				"but the claim URL is still valid for %s. Include the claim URL and the remaining time. "+
				"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
				"so they should let the agent finish its work before claiming it.\n",
			validFor)
	} else {
		return ""
	}
	message := fmt.Sprintf(
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"CLAIM_URL=%s\n",
		claimURL)
	message += expirationLine
	message += "ACTION_REQUIRED=Tell the user to claim this Pulumi agent account.\n"
	message += "INSTRUCTION=" + instruction
	return message
}

// agentLoginRequiredReason identifies why an agent must ask the user to log in.
type agentLoginRequiredReason int

const (
	// agentLoginTokenRejected means a locally unexpired agent access token was
	// rejected by the service.
	agentLoginTokenRejected agentLoginRequiredReason = iota
	// agentLoginClaimUnavailable means the service reported the stored claim
	// token is no longer claimable.
	agentLoginClaimUnavailable
)

// formatAgentLoginRequiredInstruction returns the instruction to run pulumi login
// when an ephemeral agent account can no longer authenticate.
func formatAgentLoginRequiredInstruction(
	reason agentLoginRequiredReason,
	accessTokenExpiresAt *time.Time,
	now time.Time,
) string {
	var message strings.Builder
	message.WriteString("PULUMI_EPHEMERAL_AGENT_ACCOUNT\n")
	if accessTokenExpiresAt != nil {
		fmt.Fprintf(&message,
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=%s\n",
			formatAgentClaimValidFor(*accessTokenExpiresAt, now))
	}
	message.WriteString(
		"ACTION_REQUIRED=Tell the user to run pulumi login.\n" +
			"INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate")
	switch reason {
	case agentLoginTokenRejected:
		message.WriteString(" even though local access had not expired. The account was likely claimed or revoked. " +
			"The stacks the agent was working with may have moved to the user's Pulumi account, so the agent's " +
			"existing access to those stacks may have changed. Ask the user to run pulumi login before retrying.\n")
	case agentLoginClaimUnavailable:
		message.WriteString(", and its claim URL is no longer claimable. The account was likely already claimed, expired, " +
			"or revoked. If it was claimed, the stacks the agent was working with moved to the user's Pulumi account, " +
			"so the agent's existing access to those stacks changed. Ask the user to run pulumi login before retrying.\n")
	default:
		contract.Failf("unknown agent login required reason %v", reason)
	}
	return message.String()
}

// formatAgentClaimValidFor returns a compact, approximate duration until an
// agent account or claim URL expires.
func formatAgentClaimValidFor(validUntil, now time.Time) string {
	validFor := validUntil.Sub(now)
	if validFor <= 0 {
		return "expired"
	}
	validFor = validFor.Truncate(time.Minute)
	if validFor < time.Minute {
		return "<1m"
	}

	days := int(validFor / (24 * time.Hour))
	validFor -= time.Duration(days) * 24 * time.Hour
	hours := int(validFor / time.Hour)
	validFor -= time.Duration(hours) * time.Hour
	minutes := int(validFor / time.Minute)

	var b strings.Builder
	if days > 0 {
		fmt.Fprintf(&b, "%dd", days)
	}
	if hours > 0 {
		fmt.Fprintf(&b, "%dh", hours)
	}
	if minutes > 0 || b.Len() == 0 {
		fmt.Fprintf(&b, "%dm", minutes)
	}
	return b.String()
}
