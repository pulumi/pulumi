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

const maxToolHistory = 50

type toolCallRecord struct {
	Name        string
	Args        json.RawMessage
	Result      json.RawMessage
	IsError     bool
	StartedAt   time.Time
	CompletedAt time.Time
	Pending     bool
}

func appendToolStart(history []toolCallRecord, name string, args json.RawMessage) []toolCallRecord {
	history = append(history, toolCallRecord{
		Name:      name,
		Args:      args,
		StartedAt: time.Now(),
		Pending:   true,
	})
	if len(history) > maxToolHistory {
		history = history[len(history)-maxToolHistory:]
	}
	return history
}

// completeToolCall mutates the most-recent pending entry that matches name.
// Session.runBatch dispatches calls serially, so that entry is unambiguously
// the one to complete.
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
