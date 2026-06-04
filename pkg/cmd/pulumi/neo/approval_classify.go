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

package neo

import "strings"

// approvalPhrases mirrors APPROVAL_PHRASES in the console
// (cmd/console2/src/app/agents/utils/text-approval.util.ts) and approvalPhrases
// in the service (cmd/service/services/agents/approval_classifier.go). Keep the
// shared phrases in sync so the same reply classifies identically whether it
// arrives via CLI, console, or Slack.
//
// "go on", "all right", and "alright" are CLI-only additions (not in the
// service/console lists); they only ever widen what the CLI accepts as an
// affirmative, so they're safe to carry locally.
var approvalPhrases = map[string]struct{}{
	"yes": {}, "approve": {}, "approved": {}, "ok": {}, "okay": {},
	"confirm": {}, "confirmed": {}, "go ahead": {}, "proceed": {},
	"lgtm": {}, "go for it": {}, "do it": {}, "ship it": {}, "y": {},
	// CLI-only additions:
	"go on": {}, "all right": {}, "alright": {},
}

// normalizeApprovalReply lowercases, trims, and collapses internal whitespace so
// "  Go   Ahead " matches "go ahead".
func normalizeApprovalReply(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// classifyApprovalReply maps a free-text reply to a pending approval to
// (approved, instructions). A recognized affirmative phrase approves with no
// instructions; anything else denies and forwards the typed text as the agent's
// instructions.
//
// Only whole-input affirmatives count — a compound reply like "yes but only on
// dev" stays a denial so its instructions reach the agent. We intentionally do
// not classify rejection phrases here: a bare "no" still denies with its text as
// instructions, matching prior CLI behavior.
func classifyApprovalReply(text string) (approved bool, instructions string) {
	if _, ok := approvalPhrases[normalizeApprovalReply(text)]; ok {
		return true, ""
	}
	return false, strings.TrimSpace(text)
}
