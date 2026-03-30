// Copyright 2016-2025, Pulumi Corporation.
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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	// maxSSEReconnectAttempts is the maximum number of consecutive reconnection attempts.
	maxSSEReconnectAttempts = 30
	// maxSSEReconnectDelay caps the exponential backoff delay.
	maxSSEReconnectDelay = 30 * time.Second
)

// SSEEvent represents a Server-Sent Event received from the Neo streaming endpoint.
type SSEEvent struct {
	ID    string          // event sequence number
	Event string          // event type: agent_event, heartbeat, error, task_status
	Data  json.RawMessage // raw JSON payload
}

// AgentTask represents a Neo task returned by the API.
type AgentTask struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// CreateTaskRequest is the body for POST /tasks.
type CreateTaskRequest struct {
	Message    *CreateTaskMessage `json:"message"`
	ClientMode string             `json:"clientMode,omitempty"`
}

// CreateTaskMessage is a message sent when creating a task.
type CreateTaskMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// CreateTaskResponse is returned from POST /tasks.
type CreateTaskResponse struct {
	TaskID string `json:"taskID"`
}

// ListTasksResponse is returned from GET /tasks.
type ListTasksResponse struct {
	Tasks             []AgentTask `json:"tasks"`
	ContinuationToken string      `json:"continuationToken,omitempty"`
}

// ToolResponseEvent is posted back to the service after local tool execution.
type ToolResponseEvent struct {
	Type       string `json:"type"` // "tool_response"
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// RespondToTaskRequest wraps an event sent back to the service.
type RespondToTaskRequest struct {
	Event json.RawMessage `json:"event"`
}

// UserMessageEvent is sent when the user provides additional input.
type UserMessageEvent struct {
	Type    string `json:"type"` // "user_message"
	Content string `json:"content"`
}

// UserConfirmationEvent is sent when the user approves or denies a tool call.
type UserConfirmationEvent struct {
	Type       string `json:"type"` // "user_confirmation"
	ApprovalID string `json:"approval_id"`
	Approved   bool   `json:"approved"`
}

// UpdateTaskRequest is the body for PATCH /tasks/:id.
type UpdateTaskRequest struct {
	ClientMode *string `json:"clientMode,omitempty"`
}

// NeoClient handles communication with the Neo API.
type NeoClient struct {
	apiURL     string
	apiToken   string
	httpClient *http.Client
	org        string
	userAgent  string
}

// NewNeoClient creates a new client for the Neo API.
func NewNeoClient(apiURL, apiToken, org string) *NeoClient {
	return &NeoClient{
		apiURL:   strings.TrimSuffix(apiURL, "/"),
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Default timeout for REST calls (not SSE).
		},
		org:       org,
		userAgent: "pulumi-cli/neo",
	}
}

// CreateTask creates a new Neo task with the given prompt and client mode.
func (c *NeoClient) CreateTask(ctx context.Context, prompt string, clientMode string) (string, error) {
	req := CreateTaskRequest{
		Message: &CreateTaskMessage{
			Type:    "user_message",
			Content: prompt,
		},
		ClientMode: clientMode,
	}

	var resp CreateTaskResponse
	err := c.doJSON(ctx, "POST", c.tasksURL(), req, &resp)
	if err != nil {
		return "", fmt.Errorf("creating task: %w", err)
	}
	return resp.TaskID, nil
}

// GetTask retrieves a single task by ID.
func (c *NeoClient) GetTask(ctx context.Context, taskID string) (*AgentTask, error) {
	var task AgentTask
	err := c.doJSON(ctx, "GET", c.taskURL(taskID), nil, &task)
	if err != nil {
		return nil, fmt.Errorf("getting task: %w", err)
	}
	return &task, nil
}

// ListTasks returns recent tasks for the organization.
func (c *NeoClient) ListTasks(ctx context.Context) (*ListTasksResponse, error) {
	var resp ListTasksResponse
	err := c.doJSON(ctx, "GET", c.tasksURL()+"?pageSize=20", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}
	return &resp, nil
}

// UpdateTask updates a task's client mode.
func (c *NeoClient) UpdateTask(ctx context.Context, taskID string, clientMode string) error {
	req := UpdateTaskRequest{ClientMode: &clientMode}
	return c.doJSON(ctx, "PATCH", c.taskURL(taskID), req, nil)
}

// StreamEvents opens an SSE connection and returns a channel of events.
// It handles reconnection with Last-Event-ID and exponential backoff.
func (c *NeoClient) StreamEvents(ctx context.Context, taskID string, lastEventID string) (<-chan SSEEvent, <-chan error) {
	eventCh := make(chan SSEEvent, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		currentLastID := lastEventID
		consecutiveFailures := 0

		for {
			err := c.streamOnce(ctx, taskID, currentLastID, eventCh, &currentLastID)
			if err != nil {
				if ctx.Err() != nil {
					return // context cancelled
				}

				consecutiveFailures++
				if consecutiveFailures > maxSSEReconnectAttempts {
					errCh <- fmt.Errorf("SSE reconnection failed after %d attempts: %w",
						maxSSEReconnectAttempts, err)
					return
				}

				// Exponential backoff: 1s, 2s, 4s, 8s, ... capped at maxSSEReconnectDelay.
				delay := time.Duration(1<<uint(consecutiveFailures-1)) * time.Second
				if delay > maxSSEReconnectDelay {
					delay = maxSSEReconnectDelay
				}

				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return
				}
			}
			return // Stream ended normally (server closed connection cleanly).
		}
	}()

	return eventCh, errCh
}

func (c *NeoClient) streamOnce(
	ctx context.Context, taskID string, lastEventID string,
	eventCh chan<- SSEEvent, lastID *string,
) error {
	url := c.taskURL(taskID) + "/events/stream"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "token "+c.apiToken)
	req.Header.Set("User-Agent", c.userAgent)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	// Use a separate client for SSE without the default timeout.
	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SSE stream returned %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer size for large events.
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var currentEvent SSEEvent
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line = end of event. Per SSE spec, concatenate data lines with newlines.
			if len(dataLines) > 0 {
				currentEvent.Data = json.RawMessage(strings.Join(dataLines, "\n"))
			}
			// Always update the last event ID when present, even for
			// id-only messages with no data. The server uses id-only
			// messages to communicate the pagination cursor for
			// reconnection via Last-Event-ID.
			if currentEvent.ID != "" {
				*lastID = currentEvent.ID
			}
			if currentEvent.Data != nil {
				select {
				case eventCh <- currentEvent:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			currentEvent = SSEEvent{}
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, ":") {
			// SSE comment line, skip.
			continue
		}

		// Parse field:value, per SSE spec the space after colon is optional.
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		field := line[:colonIdx]
		value := line[colonIdx+1:]
		// Remove optional leading space after colon.
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		switch field {
		case "id":
			currentEvent.ID = value
		case "event":
			currentEvent.Event = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}

	return scanner.Err()
}

// PostToolResponse posts a tool execution result back to the service via the events endpoint.
// Tool responses are posted as backend events since they come from local execution.
func (c *NeoClient) PostToolResponse(ctx context.Context, taskID string, result ToolResponseEvent) error {
	return c.doJSON(ctx, "POST", c.taskURL(taskID)+"/events", result, nil)
}

// PostUserInput sends a user message or confirmation via the RespondToTask endpoint.
func (c *NeoClient) PostUserInput(ctx context.Context, taskID string, event interface{}) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req := RespondToTaskRequest{Event: eventBytes}
	return c.doJSON(ctx, "POST", c.taskURL(taskID), req, nil)
}

// ConsoleTaskURL returns the Pulumi Cloud console URL for a task, or "" if the
// console domain cannot be determined from the API URL.
func (c *NeoClient) ConsoleTaskURL(taskID string) string {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return ""
	}

	switch {
	case os.Getenv("PULUMI_CONSOLE_DOMAIN") != "":
		u.Host = os.Getenv("PULUMI_CONSOLE_DOMAIN")
	case strings.HasPrefix(u.Host, "api."):
		// api.pulumi.com → app.pulumi.com, api.pulumi-dev.io → app.pulumi-dev.io, etc.
		u.Host = "app." + u.Host[len("api."):]
	case strings.HasPrefix(u.Host, "api-"):
		// api-joeduffy.review-stacks.pulumi-dev.io → app-joeduffy.review-stacks.pulumi-dev.io
		u.Host = "app-" + u.Host[len("api-"):]
	case u.Host == "localhost:8080":
		u.Host = "localhost:3000"
	default:
		return ""
	}

	u.Path = path.Join(c.org, "neo", "tasks", taskID)
	return u.String()
}

func (c *NeoClient) tasksURL() string {
	return fmt.Sprintf("%s/api/preview/agents/%s/tasks", c.apiURL, c.org)
}

func (c *NeoClient) taskURL(taskID string) string {
	return fmt.Sprintf("%s/api/preview/agents/%s/tasks/%s", c.apiURL, c.org, taskID)
}

func (c *NeoClient) doJSON(ctx context.Context, method, url string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
