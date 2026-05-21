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
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// MaybePrintClaimWarning reminds detected coding agents to tell the user about
// a claim URL for shared agent credentials used by this CLI process.
func MaybePrintClaimWarning() {
	if agentdetect.Detect(os.Getenv) == "" {
		return
	}

	now := time.Now()
	deleted, err := workspace.DeleteExpiredAgentCredentials(now)
	if err != nil {
		logging.V(7).Infof("Could not delete expired agent credentials: %v", err)
		return
	}
	if deleted {
		return
	}

	claim, err := workspace.GetAgentClaim()
	if err != nil || claim.ClaimURL == "" {
		return
	}
	if claim.CloudURL == "" || !httpstate.AgentCredentialsUsed(claim.CloudURL) {
		return
	}
	account, err := workspace.GetAgentAccount(claim.CloudURL)
	if err != nil {
		logging.V(7).Infof("Could not read agent account credentials: %v", err)
		return
	}
	var accessTokenExpiresAt *time.Time
	if account.TokenInformation != nil {
		accessTokenExpiresAt = account.TokenInformation.ExpiresAt
	}
	if (accessTokenExpiresAt == nil || !accessTokenExpiresAt.After(now)) &&
		(claim.ValidUntil.IsZero() || !claim.ValidUntil.After(now)) {
		return
	}

	warning := workspace.FormatAgentClaimInstruction(claim.ClaimURL, accessTokenExpiresAt, claim.ValidUntil, now)
	_, err = os.Stderr.WriteString(warning)
	contract.IgnoreError(err)
}

// AuthRequiredMessage returns the structured instruction shown to coding
// agents when an ephemeral agent account can no longer authenticate. If the
// local token has already expired but the claim URL is still valid, it returns
// the claim instruction instead of the auth-required instruction.
func AuthRequiredMessage(now time.Time) string {
	if agentdetect.Detect(os.Getenv) == "" {
		return ""
	}

	claim, err := workspace.GetAgentClaim()
	if err != nil || claim.CloudURL == "" {
		return ""
	}
	account, err := workspace.GetAgentAccount(claim.CloudURL)
	if err != nil {
		return ""
	}
	expiresAt, valid := workspace.AgentAccessTokenExpiresAt(account, now)
	if valid && expiresAt != nil {
		return workspace.FormatAgentAuthRequiredInstruction(*expiresAt, now)
	}
	return workspace.FormatAgentClaimInstruction(claim.ClaimURL, expiresAt, claim.ValidUntil, now)
}
