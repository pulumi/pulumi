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

type SummarizeUpdate struct {
	Summary string `json:"summary"`
}

type ThreadMessage struct {
	Role    string          `json:"role"`
	Kind    string          `json:"kind"`
	Content json.RawMessage `json:"content"`
}

type AtlasUpdateSummaryResponse struct {
	ThreadMessages []ThreadMessage `json:"messages"`
	Error          string          `json:"error"`
	Details        any             `json:"details"`
}

type CloudContext struct {
	OrgID string `json:"orgId"`
	URL   string `json:"url"`
}

type ClientState struct {
	CloudContext CloudContext `json:"cloudContext"`
}

type State struct {
	Client ClientState `json:"client"`
}

type SkillParams struct {
	PulumiUpdateOutput string `json:"pulumiUpdateOutput"`
}

type DirectSkillCall struct {
	Skill  string      `json:"skill"`
	Params SkillParams `json:"params"`
}

type AtlasUpdateSummaryRequest struct {
	Query           string          `json:"query"`
	State           State           `json:"state"`
	DirectSkillCall DirectSkillCall `json:"directSkillCall"`
}
