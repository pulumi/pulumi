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

import (
	"encoding/json"
	"time"
)

// maxToolHistory bounds the number of records kept in Model.toolHistory so a
// very long session can't grow the slice without limit. The oldest entries
// drop off the front when this is exceeded.
const maxToolHistory = 50

// toolCallRecord is one entry in the overlay's tool-call history. Args are
// captured at start time; Result and CompletedAt land when the matching
// UIToolCompleted arrives. While Pending is true, Result is empty and the
// overlay renders an "(in flight)" placeholder.
type toolCallRecord struct {
	Name        string
	Args        json.RawMessage
	Result      json.RawMessage
	IsError     bool
	StartedAt   time.Time
	CompletedAt time.Time
	Pending     bool
}

// appendToolStart records a new in-flight tool call. The caller drops the
// oldest entry when the slice exceeds maxToolHistory.
func appendToolStart(history []toolCallRecord, name string, args json.RawMessage) []toolCallRecord {
	rec := toolCallRecord{
		Name:      name,
		Args:      args,
		StartedAt: time.Now(),
		Pending:   true,
	}
	history = append(history, rec)
	if len(history) > maxToolHistory {
		history = history[len(history)-maxToolHistory:]
	}
	return history
}

// completeToolCall mutates the most recent pending entry that matches name to
// reflect a completed call. runBatch dispatches calls serially so matching on
// (name, most-recent-pending) is unambiguous in practice. If no match is
// found (e.g. completed event with no matching start) the history is left
// untouched.
func completeToolCall(history []toolCallRecord, name string, result json.RawMessage, isError bool) {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Pending && history[i].Name == name {
			history[i].Result = result
			history[i].IsError = isError
			history[i].CompletedAt = time.Now()
			history[i].Pending = false
			return
		}
	}
}
