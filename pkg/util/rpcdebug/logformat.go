// Copyright 2016-2022, Pulumi Corporation.
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

package rpcdebug

import (
	"encoding/json"
	"time"
)

// JSON format for tracking gRPC conversations. Normal methods have
// two entries for each request-response interaction. Streaming methods have
// one entry per each request or response over the stream.
type debugInterceptorLogEntry struct {
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Errors   []string        `json:"errors,omitempty"`
	Metadata any             `json:"metadata,omitempty"`
	// Indicates the state of RPC methods, can be "request_started" or "response_completed"
	Progress  string        `json:"progress,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}
