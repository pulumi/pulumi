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
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolHandler executes a single named method on a Neo CLI-local tool. The method is the
// part of the agent's full tool name after the "<server>__" prefix; args is the raw JSON
// arguments object. The returned value is JSON-encoded into the ToolResultItem content.
type ToolHandler interface {
	Invoke(ctx context.Context, method string, args json.RawMessage) (any, error)
}

// Executor dispatches ToolCalls to ToolHandlers keyed by server name (the part of the
// tool name before "__"). Per-call failures become ToolResultItems with IsError=true
// rather than aborting the batch — the agent can then either retry or report the failure
// to the user.
type Executor struct {
	handlers map[string]ToolHandler
}

// NewExecutor creates an empty Executor. Register handlers with Register before use.
func NewExecutor() *Executor {
	return &Executor{handlers: map[string]ToolHandler{}}
}

// Register associates a server name (e.g. "filesystem") with a ToolHandler.
func (e *Executor) Register(server string, h ToolHandler) {
	e.handlers[server] = h
}

// Execute runs every call in the batch sequentially and returns the result items to be
// embedded in a ToolResultEvent. The caller wraps the items in the envelope and posts them.
func (e *Executor) Execute(ctx context.Context, calls []ToolCall) []ToolResultItem {
	out := make([]ToolResultItem, 0, len(calls))
	for _, call := range calls {
		out = append(out, e.invoke(ctx, call))
	}
	return out
}

func (e *Executor) invoke(ctx context.Context, call ToolCall) ToolResultItem {
	res := ToolResultItem{ToolCallID: call.ToolCallID, Name: call.Name}

	server, method, ok := strings.Cut(call.Name, "__")
	if !ok {
		res.IsError = true
		res.Content = map[string]string{"error": fmt.Sprintf("tool name %q is missing the server prefix", call.Name)}
		return res
	}
	handler, ok := e.handlers[server]
	if !ok {
		res.IsError = true
		res.Content = map[string]string{"error": fmt.Sprintf("tool %q is not available in CLI mode", server)}
		return res
	}
	value, err := handler.Invoke(ctx, method, call.Args)
	if err != nil {
		res.IsError = true
		res.Content = map[string]string{"error": err.Error()}
		return res
	}
	// Surface marshalability errors per-call instead of failing the whole batch at POST time.
	if _, err := json.Marshal(value); err != nil {
		res.IsError = true
		res.Content = map[string]string{"error": fmt.Sprintf("encoding result: %v", err)}
		return res
	}
	res.Content = value
	return res
}
