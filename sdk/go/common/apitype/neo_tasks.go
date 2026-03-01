// Copyright 2016-2026, Pulumi Corporation.
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

import "time"

// CreateNeoTaskRequest represents the request body for creating a Neo task.
type CreateNeoTaskRequest struct {
	Message NeoTaskMessage `json:"message"`
}

// NeoTaskMessage represents a message in a Neo task.
// This needs to match the structure expected by the Pulumi Service API.
type NeoTaskMessage struct {
	Type      string         `json:"type"` // Should be "user_message"
	Timestamp time.Time      `json:"timestamp"`
	Content   string         `json:"content"`
	Commands  map[string]any `json:"commands,omitempty"`
}

// CreateNeoTaskResponse represents the response from creating a Neo task.
type CreateNeoTaskResponse struct {
	TaskID string `json:"taskID"`
}
