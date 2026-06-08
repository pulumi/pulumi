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

// affirmativeReplies are the whole-input replies the CLI accepts as an approval.
// The shared phrases mirror the console "Yes" button and the service classifier
// (pulumi-service cmd/service/services/agents/approval_classifier.go) — keep them
// in sync so a reply classifies the same via CLI, console, or Slack. "go on",
// "all right" and "alright" are CLI-only; they only widen what counts as a yes.
var affirmativeReplies = map[string]struct{}{
	"yes": {}, "y": {}, "ok": {}, "okay": {}, "approve": {}, "approved": {},
	"confirm": {}, "confirmed": {}, "proceed": {}, "go ahead": {}, "go for it": {},
	"do it": {}, "ship it": {}, "lgtm": {}, "go on": {}, "all right": {}, "alright": {},
}

// isAffirmative reports whether an approval reply is a bare yes, case- and
// whitespace-insensitive ("  Go  Ahead " == "go ahead"). A compound reply like
// "yes but only on dev" is not affirmative, so its text still reaches the agent
// as instructions.
func isAffirmative(reply string) bool {
	_, ok := affirmativeReplies[strings.Join(strings.Fields(strings.ToLower(reply)), " ")]
	return ok
}
