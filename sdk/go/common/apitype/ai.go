// Copyright 2016-2018, Pulumi Corporation.
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
	OrgID string `json:"orgId"`
	URL   string `json:"url"`
}

type CopilotDirectSkillCall struct {
	Skill  string             `json:"skill"`
	Params CopilotSkillParams `json:"params"`
}

type CopilotSkillParams struct {
	PulumiUpdateOutput string `json:"pulumiUpdateOutput"`
	Model              string `json:"model"`
	MaxLen             int    `json:"maxLen"`
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
