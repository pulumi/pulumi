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
	Model              string `json:"model,omitempty"`    // The model to use for the summary.
	MaxLen             int    `json:"maxLen,omitempty"`   // The maximum length of the summary.
}

// Responses

type CopilotSummarizeUpdateResponse struct {
	ThreadMessages []CopilotThreadMessage `json:"messages"`
	Error          string                 `json:"error"`
	Details        any                    `json:"details"`
}

type CopilotThreadMessage struct {
	Role    string          `json:"role"`
	Kind    string          `json:"kind"`
	Content json.RawMessage `json:"content"`
}

type CopilotSummarizeUpdateMessage struct {
	Summary string `json:"summary"`
}
