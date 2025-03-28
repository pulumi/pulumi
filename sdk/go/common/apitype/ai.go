// Copyright 2025, Pulumi Corporation.
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

import "encoding/json"

// Requests

type CopilotSummarizeUpdateRequest struct {
	Query           string                 `json:"query"`
	State           CopilotState           `json:"state"`
	DirectSkillCall CopilotDirectSkillCall `json:"directSkillCall"`
}

type CopilotState struct {
	Client CopilotClientState `json:"client"`
}

type CopilotClientState struct {
	CloudContext CopilotCloudContext `json:"cloudContext"`
}

type CopilotCloudContext struct {
	OrgID string `json:"orgId"` // The organization ID.
	URL   string `json:"url"`   // The URL the user is viewing. Mock value often used.
}

type CopilotDirectSkillCall struct {
	Skill  string             `json:"skill"` // The skill to call. e.g. "summarizeUpdate"
	Params CopilotSkillParams `json:"params"`
}

type CopilotSkillParams struct {
	PulumiUpdateOutput string `json:"pulumiUpdateOutput"` // The Pulumi update output to summarize.
	Model              string `json:"model,omitempty"`    // The model to use for the summary. e.g. "gpt-4o-mini"
	MaxLen             int    `json:"maxLen,omitempty"`   // The maximum length of the returned summary.
}

// Responses

type CopilotSummarizeUpdateResponse struct {
	ThreadMessages []CopilotThreadMessage `json:"messages"`
	Error          string                 `json:"error"`
	Details        string                 `json:"details"` // The details of the error.
}

type CopilotThreadMessage struct {
	Role    string          `json:"role"`    // The role of the message. e.g. "assistant" / "user"
	Kind    string          `json:"kind"`    // Depends on the tool called, e.g. "response" / "program"
	Content json.RawMessage `json:"content"` // The content of the message. String or JSON object.
}
