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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type permissionStreamer struct {
	*fakeStreamer
	getTask func(context.Context, string, string) (*client.NeoTask, error)
}

func (s *permissionStreamer) GetNeoTask(ctx context.Context, org, taskID string) (*client.NeoTask, error) {
	return s.getTask(ctx, org, taskID)
}

type permissionHandler func(context.Context, string, json.RawMessage) (any, error)

func (h permissionHandler) Invoke(ctx context.Context, method string, args json.RawMessage) (any, error) {
	return h(ctx, method, args)
}

func TestSession_DeploymentPermission(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		mode      client.NeoPermissionMode
		err       error
		wantError string
	}{
		{name: "read only", mode: client.NeoPermissionModeReadOnly, wantError: "read-only"},
		{name: "default", mode: client.NeoPermissionModeDefault},
		{name: "missing mode", wantError: "permission mode"},
		{name: "unknown mode", mode: "future-mode", wantError: "permission mode"},
		{name: "lookup failure", err: errors.New("offline"), wantError: "offline"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			invoked := false
			streamer := &permissionStreamer{
				fakeStreamer: newFakeStreamer(),
				getTask: func(_ context.Context, org, taskID string) (*client.NeoTask, error) {
					assert.Equal(t, "org", org)
					assert.Equal(t, "task", taskID)
					return &client.NeoTask{PermissionMode: tt.mode}, tt.err
				},
			}
			s := &Session{
				Client: streamer, OrgName: "org", TaskID: "task",
				Handlers: map[string]ToolHandler{"pulumi": permissionHandler(
					func(_ context.Context, method string, _ json.RawMessage) (any, error) {
						invoked = true
						assert.Equal(t, "pulumi_up", method)
						return "deployed", nil
					})},
			}
			result := s.invokeToolCall(t.Context(), apitype.AgentBackendEventToolCall{
				ToolCallID: "up-1", Name: "pulumi__pulumi_up", Args: json.RawMessage(`{}`),
			})
			assert.Equal(t, "up-1", result.ToolCallID)
			assert.Equal(t, "pulumi__pulumi_up", result.Name)
			if tt.wantError != "" {
				assert.False(t, invoked, "denied deployments must not reach the handler")
				require.True(t, result.IsError)
				assert.Contains(t, result.Content.(map[string]string)["error"], tt.wantError)
			} else {
				assert.True(t, invoked)
				assert.False(t, result.IsError)
				assert.Equal(t, "deployed", result.Content)
			}
		})
	}
}

func TestSession_ReadOnlyMixedBatchPostsEveryResult(t *testing.T) {
	t.Parallel()

	streamer := &permissionStreamer{
		fakeStreamer: newFakeStreamer(),
		getTask: func(context.Context, string, string) (*client.NeoTask, error) {
			return &client.NeoTask{PermissionMode: client.NeoPermissionModeReadOnly}, nil
		},
	}
	var invoked []string
	handler := permissionHandler(func(_ context.Context, method string, _ json.RawMessage) (any, error) {
		invoked = append(invoked, method)
		return "ok", nil
	})
	s := &Session{
		Client: streamer, OrgName: "org", TaskID: "task",
		Handlers: map[string]ToolHandler{"pulumi": handler, "filesystem": handler},
	}
	calls := []apitype.AgentBackendEventToolCall{
		{ToolCallID: "up-1", Name: "pulumi__pulumi_up", ExecutionMode: "cli"},
		{ToolCallID: "preview", Name: "pulumi__pulumi_preview", ExecutionMode: "cli"},
		{ToolCallID: "up-2", Name: "pulumi__pulumi_up", ExecutionMode: "cli"},
		{ToolCallID: "write", Name: "filesystem__write_file", ExecutionMode: "cli"},
	}
	streamer.stream <- client.NeoStreamEvent{Data: mustAgentResponseEnvelope(t,
		apitype.AgentBackendEventAssistantMessage{
			Type: backendEventAssistantMessage, IsFinal: true, ToolCalls: calls,
		})}
	close(streamer.stream)
	require.NoError(t, s.Run(t.Context()))
	assert.Equal(t, []string{"pulumi_preview", "write_file"}, invoked)
	require.Len(t, streamer.posted, len(calls)+1)
	for i, call := range calls {
		execEvent, ok := streamer.posted[i].(apitype.AgentUserEventExecToolCall)
		require.True(t, ok)
		assert.Equal(t, call.ToolCallID, execEvent.ToolCallID)
	}
	result, ok := streamer.posted[len(calls)].(apitype.AgentUserEventToolResult)
	require.True(t, ok)
	require.Len(t, result.ToolResults, len(calls))
	for i, call := range calls {
		assert.Equal(t, call.ToolCallID, result.ToolResults[i].ToolCallID)
		assert.Equal(t, i == 0 || i == 2, result.ToolResults[i].IsError)
	}
}

func TestSession_DeploymentRechecksPermissionWithinBatch(t *testing.T) {
	t.Parallel()

	mode := client.NeoPermissionModeDefault
	streamer := &permissionStreamer{
		fakeStreamer: newFakeStreamer(),
		getTask: func(context.Context, string, string) (*client.NeoTask, error) {
			return &client.NeoTask{PermissionMode: mode}, nil
		},
	}
	deployments := 0
	s := &Session{
		Client: streamer,
		Handlers: map[string]ToolHandler{"pulumi": permissionHandler(
			func(context.Context, string, json.RawMessage) (any, error) {
				deployments++
				mode = client.NeoPermissionModeReadOnly
				return "deployed", nil
			})},
	}
	require.NoError(t, s.runBatch(t.Context(), []apitype.AgentBackendEventToolCall{
		{ToolCallID: "up-1", Name: "pulumi__pulumi_up"},
		{ToolCallID: "up-2", Name: "pulumi__pulumi_up"},
	}))
	assert.Equal(t, 1, deployments)
	result := streamer.posted[2].(apitype.AgentUserEventToolResult)
	require.Len(t, result.ToolResults, 2)
	assert.False(t, result.ToolResults[0].IsError)
	assert.True(t, result.ToolResults[1].IsError)
}

func TestSession_NonDeploymentDoesNotNeedPermissionLookup(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"pulumi__pulumi_preview", "filesystem__write_file"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			streamer := &permissionStreamer{
				fakeStreamer: newFakeStreamer(),
				getTask: func(context.Context, string, string) (*client.NeoTask, error) {
					t.Error("non-deployment tool must not need permission metadata")
					return nil, errors.New("offline")
				},
			}
			handler := permissionHandler(func(context.Context, string, json.RawMessage) (any, error) {
				return "ok", nil
			})
			s := &Session{Client: streamer, Handlers: map[string]ToolHandler{"pulumi": handler, "filesystem": handler}}
			result := s.invokeToolCall(t.Context(), apitype.AgentBackendEventToolCall{Name: name})
			assert.False(t, result.IsError)
			assert.Equal(t, "ok", result.Content)
		})
	}
}

func TestSession_DeploymentCancelledDuringPermissionLookup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	streamer := &permissionStreamer{
		fakeStreamer: newFakeStreamer(),
		getTask: func(context.Context, string, string) (*client.NeoTask, error) {
			cancel()
			return &client.NeoTask{PermissionMode: client.NeoPermissionModeDefault}, nil
		},
	}
	s := &Session{
		Client: streamer,
		Handlers: map[string]ToolHandler{"pulumi": permissionHandler(
			func(context.Context, string, json.RawMessage) (any, error) {
				t.Error("cancelled deployment must not reach the handler")
				return "deployed", nil
			})},
	}
	result := s.invokeToolCall(ctx, apitype.AgentBackendEventToolCall{Name: "pulumi__pulumi_up"})
	assert.True(t, result.IsError)
	assert.Equal(t, cancelledContent(), result.Content)
}
