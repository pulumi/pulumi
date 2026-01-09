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

// Base request type for Neo requests.

type CopilotRequest struct {
	Query string       `json:"query"`
	State CopilotState `json:"state"`
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

// CopilotSkill is the Neo "direct skill call" to be used in the request.
type CopilotSkill string

const (
	SkillSummarizeUpdate CopilotSkill = "summarizeUpdate"
	SkillExplainPreview  CopilotSkill = "explainPreview"
)

// SummarizeUpdateRequest

type CopilotSummarizeUpdateRequest struct {
	CopilotRequest
	DirectSkillCall CopilotSummarizeUpdate `json:"directSkillCall"`
}

type CopilotSummarizeUpdate struct {
	Skill  CopilotSkill                 `json:"skill"` // Always "summarizeUpdate"
	Params CopilotSummarizeUpdateParams `json:"params"`
}

type CopilotSummarizeUpdateParams struct {
	PulumiUpdateOutput string `json:"pulumiUpdateOutput"` // The Pulumi update output to summarize.
	Model              string `json:"model,omitempty"`    // The model to use for the summary. e.g. "gpt-4o-mini"
	MaxLen             int    `json:"maxLen,omitempty"`   // The maximum length of the returned summary.
}

// ExplainPreviewRequest

type CopilotExplainPreviewRequest struct {
	CopilotRequest
	DirectSkillCall CopilotExplainPreviewSkill `json:"directSkillCall"`
}

type CopilotExplainPreviewSkill struct {
	Skill  CopilotSkill                `json:"skill"` // Always "explainPreview"
	Params CopilotExplainPreviewParams `json:"params"`
}

type CopilotExplainPreviewParams struct {
	// The Pulumi preview output to explain.
	PulumiPreviewOutput string `json:"pulumiPreviewOutput"`
	// The details of the preview.
	PreviewDetails CopilotExplainPreviewDetails `json:"previewDetails"`
}

type CopilotExplainPreviewDetails struct {
	Kind string `json:"kind"` // The kind of update that is being explained. e.g. update, refresh, destroy, etc.
}

// Responses

type CopilotResponse struct {
	ThreadMessages []CopilotThreadMessage `json:"messages"`
	Error          string                 `json:"error"`
	Details        string                 `json:"details"` // The details of the error.
}

type CopilotThreadMessage struct {
	Role    string          `json:"role"`    // The role of the message. e.g. "assistant" / "user"
	Kind    string          `json:"kind"`    // Depends on the tool called, e.g. "response" / "program"
	Content json.RawMessage `json:"content"` // The content of the message. String or JSON object.
}

// Neo Task Types

// NeoEntity represents an entity associated with a Neo task.
type NeoEntity struct {
	Type    string `json:"type"`              // Entity type (e.g., "stack", "repository")
	Name    string `json:"name,omitempty"`    // Entity name
	Project string `json:"project,omitempty"` // Project name (for stacks)
	ID      string `json:"id,omitempty"`      // Entity ID
}

// NeoEntityDiff represents entities to add or remove from a task.
type NeoEntityDiff struct {
	Add    []NeoEntity `json:"add,omitempty"`    // Entities to add
	Remove []NeoEntity `json:"remove,omitempty"` // Entities to remove
}

// NeoMessage represents a user message for Neo.
type NeoMessage struct {
	Type       string         `json:"type"`                  // Message type (always "user_message")
	Content    string         `json:"content"`               // User's message content
	Timestamp  string         `json:"timestamp"`             // ISO 8601 timestamp
	EntityDiff *NeoEntityDiff `json:"entity_diff,omitempty"` // Optional entity changes
}

// NeoTaskRequest is the request to create a new Neo task.
type NeoTaskRequest struct {
	Message NeoMessage `json:"message"` // The user's message
}

// NeoTask represents a Neo task.
type NeoTask struct {
	ID        string      `json:"id"`        // The unique ID of the task
	Name      string      `json:"name"`      // Human-readable task name
	Status    string      `json:"status"`    // Status: "running" or "idle"
	CreatedAt string      `json:"createdAt"` // When the task was created (ISO 8601)
	Entities  []NeoEntity `json:"entities"`  // Associated entities
}

// NeoTaskResponse is the response from creating a Neo task.
type NeoTaskResponse struct {
	TaskID string `json:"taskId"` // The created task ID
}

// NeoTaskListResponse is the response from listing Neo tasks.
type NeoTaskListResponse struct {
	Tasks             []NeoTask `json:"tasks"`                       // The list of tasks
	ContinuationToken string    `json:"continuationToken,omitempty"` // Token for pagination
}
