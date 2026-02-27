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
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ValidApprovalModes defines the recognized values for --approval-mode.
var ValidApprovalModes = []string{"auto", "balanced", "manual"}

// validateApprovalMode checks that the approval mode is a recognized value.
func validateApprovalMode(mode string) error {
	for _, valid := range ValidApprovalModes {
		if mode == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid approval mode %q; must be one of: %s",
		mode, strings.Join(ValidApprovalModes, ", "))
}

// getBackendInfo extracts the API URL, access token, organization, and username
// from the current Pulumi Cloud credentials. The orgFlag overrides automatic org
// resolution. Respects PULUMI_BACKEND_URL to override the current backend.
func getBackendInfo(ctx context.Context, orgFlag string) (apiURL, apiToken, org, username string, err error) {
	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return "", "", "", "", fmt.Errorf("not logged in: %w", err)
	}

	// Allow PULUMI_BACKEND_URL to override the current backend, matching the behavior
	// of the rest of the CLI (see workspace.GetCurrentCloudURL).
	apiURL = os.Getenv("PULUMI_BACKEND_URL")
	if apiURL == "" {
		apiURL = creds.Current
	}
	if apiURL == "" {
		return "", "", "", "", fmt.Errorf("no current backend, please run 'pulumi login'")
	}

	apiToken = creds.AccessTokens[apiURL]
	if apiToken == "" {
		return "", "", "", "", fmt.Errorf("no access token for %s, please run 'pulumi login'", apiURL)
	}

	if orgFlag != "" {
		org = orgFlag
	}

	// Try to resolve the org and username from the stored account info.
	if account, ok := creds.Accounts[apiURL]; ok && account.Username != "" {
		username = account.Username
		if org == "" {
			org = account.Username
		}
	}

	if org == "" {
		return "", "", "", "", fmt.Errorf("could not determine organization; use --org to specify one")
	}

	return apiURL, apiToken, org, username, nil
}

// runNeo is the main entry point for `pulumi neo [prompt]`.
// It creates a new task on the service and begins the event loop.
func runNeo(ctx context.Context, prompt string, orgFlag string, interactive bool, jsonOutput bool,
	approvalMode string,
) error {
	if err := validateApprovalMode(approvalMode); err != nil {
		return err
	}

	apiURL, apiToken, org, username, err := getBackendInfo(ctx, orgFlag)
	if err != nil {
		return err
	}

	client := NewNeoClient(apiURL, apiToken, org)
	display := NewDisplay(os.Stdout, os.Stderr, os.Stdin, interactive, jsonOutput)
	display.SetApprovalMode(&approvalMode)

	clientMode := "cli"
	if !interactive {
		clientMode = "cli-noninteractive"
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Show the welcome banner before prompting (without session link).
	if !jsonOutput {
		display.RenderWelcome(org, workDir, username, "")
	}

	if prompt == "" && interactive {
		// Interactive mode — prompt for the initial message before creating the session.
		input, err := display.PromptUserInput()
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		prompt = input
		if prompt == "" {
			return nil
		}
	}

	taskID, err := client.CreateTask(ctx, prompt, clientMode)
	if err != nil {
		return err
	}

	// Redraw the welcome banner with the session link now that we have a task ID.
	consoleURL := client.ConsoleTaskURL(taskID)
	if !jsonOutput {
		display.RedrawWelcome(consoleURL)
	}

	// Render the styled initial user message below the banner.
	if interactive {
		display.RenderUserMessage(prompt)
	}
	display.StartThinking()

	executor := NewToolExecutor(workDir, func(req ApprovalRequest) (bool, error) {
		return display.RequestApproval(req.Message)
	}, func(change FileChange) {
		display.RenderDiff(change)
	})

	return runEventLoop(ctx, client, taskID, executor, display, interactive, &approvalMode)
}

// runAttach attaches to an existing Neo session by task ID.
func runAttach(ctx context.Context, taskID string, orgFlag string, interactive bool, jsonOutput bool,
	approvalMode string,
) error {
	if err := validateApprovalMode(approvalMode); err != nil {
		return err
	}

	apiURL, apiToken, org, _, err := getBackendInfo(ctx, orgFlag)
	if err != nil {
		return err
	}

	client := NewNeoClient(apiURL, apiToken, org)
	display := NewDisplay(os.Stdout, os.Stderr, os.Stdin, interactive, jsonOutput)
	display.SetApprovalMode(&approvalMode)

	// Update the task's client mode to indicate a CLI client is now attached.
	clientMode := "cli"
	if !interactive {
		clientMode = "cli-noninteractive"
	}
	if err := client.UpdateTask(ctx, taskID, clientMode); err != nil {
		return fmt.Errorf("attaching to session: %w", err)
	}

	consoleURL := client.ConsoleTaskURL(taskID)
	if !jsonOutput {
		display.RenderSessionAttach(taskID, consoleURL)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	executor := NewToolExecutor(workDir, func(req ApprovalRequest) (bool, error) {
		return display.RequestApproval(req.Message)
	}, func(change FileChange) {
		display.RenderDiff(change)
	})

	return runEventLoop(ctx, client, taskID, executor, display, interactive, &approvalMode)
}

// runListSessions lists recent Neo sessions for the organization.
func runListSessions(ctx context.Context, orgFlag string) error {
	apiURL, apiToken, org, _, err := getBackendInfo(ctx, orgFlag)
	if err != nil {
		return err
	}

	client := NewNeoClient(apiURL, apiToken, org)

	resp, err := client.ListTasks(ctx)
	if err != nil {
		return err
	}

	if len(resp.Tasks) == 0 {
		fmt.Println("No recent sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCREATED")
	for _, t := range resp.Tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, t.Name, t.Status, t.CreatedAt)
	}
	return w.Flush()
}

// toolResult carries the result of an asynchronous tool execution.
type toolResult struct {
	result ToolResponseEvent
	err    error // error posting the result back to the service
}

// postToolResponseWithRetry posts a tool response to the service with retry logic.
// It retries up to maxRetries times with exponential backoff on failure.
func postToolResponseWithRetry(
	ctx context.Context, client *NeoClient, taskID string,
	result ToolResponseEvent, display *Display,
) error {
	const maxRetries = 3
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = client.PostToolResponse(ctx, taskID, result)
		if err == nil {
			return nil
		}
		display.LogDebug("SESSION: PostToolResponse attempt %d failed for id=%s: %v",
			attempt+1, result.ToolCallID, err)
		if attempt < maxRetries-1 {
			delay := time.Duration(1<<uint(attempt)) * time.Second // 1s, 2s
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return err
}

// runEventLoop connects to the SSE stream and processes events, dispatching
// tool calls to the executor, rendering output via the display, and handling
// user interaction for approvals and multi-turn conversation.
func runEventLoop(ctx context.Context, client *NeoClient, taskID string,
	executor *ToolExecutor, display *Display, interactive bool, approvalMode *string,
) error {
	// Derive a cancellable context so all background goroutines (SSE reader,
	// tool executors) are cleaned up when the event loop exits for any reason.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events, errCh := client.StreamEvents(ctx, taskID, "")

	// Tool execution results flow through two stages:
	//   1. toolExecCh: goroutines execute tools concurrently and send results here
	//   2. toolPosterDoneCh: the poster goroutine reads from toolExecCh, posts
	//      results to the server SEQUENTIALLY (with retry), and notifies here
	//
	// Serializing the HTTP posts is critical: concurrent PostToolResponse calls
	// cause concurrent DB transactions that can commit out of order, leading to
	// AUTO_INCREMENT sequence gaps. The SSE poll uses cursor-based pagination
	// on the sequence column, so out-of-order commits can cause events to be
	// permanently skipped — hanging the session.
	toolExecCh := make(chan ToolResponseEvent, 16)
	toolPosterDoneCh := make(chan toolResult, 16)

	// Poster goroutine: reads tool execution results and posts them to the
	// server one at a time with retry. This ensures events are committed in
	// sequence order, preventing the AUTO_INCREMENT gap-skipping bug.
	go func() {
		defer close(toolPosterDoneCh)
		for {
			select {
			case result, ok := <-toolExecCh:
				if !ok {
					return
				}
				display.LogDebug("SESSION: posting tool_response id=%s", result.ToolCallID)
				postErr := postToolResponseWithRetry(ctx, client, taskID, result, display)
				display.LogDebug("SESSION: post complete id=%s post_err=%v", result.ToolCallID, postErr)
				select {
				case toolPosterDoneCh <- toolResult{result: result, err: postErr}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Enable the persistent input bar for interactive TTY sessions.
	display.EnableInputBar()
	defer display.DisableInputBar()

	// Track whether the agent is idle (waiting for user input).
	agentIdle := false
	ctrlCPending := false // true after first Ctrl+C; second Ctrl+C exits

	for {
		// Use the nil channel pattern: only select on inputCh when the agent is idle.
		var inputCh <-chan string
		if agentIdle {
			inputCh = display.InputCh()
		}

		// If input bar is not active (non-TTY), fall back to blocking prompt.
		if agentIdle && inputCh == nil {
			// Blocking path for non-TTY / non-interactive.
			input, err := display.PromptUserInput()
			if err != nil {
				return err
			}
			if input == "" {
				agentIdle = false
				continue
			}
			if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
				return nil
			}
			display.RenderUserMessage(input)
			msg := UserMessageEvent{
				Type:    "user_message",
				Content: input,
			}
			if err := client.PostUserInput(ctx, taskID, msg); err != nil {
				return fmt.Errorf("posting user input: %w", err)
			}
			display.StartThinking()
			agentIdle = false
			ctrlCPending = false
			continue
		}

		// Always monitor the cancel channel so Ctrl+C/D work even when the agent is busy.
		cancelCh := display.CancelCh()

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-cancelCh:
			if ctrlCPending {
				return nil
			}
			ctrlCPending = true
			display.RenderWarning("Ctrl+C detected; press again to quit.")
			continue

		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("stream error: %w", err)
			}

		case tr := <-toolPosterDoneCh:
			display.LogDebug("SESSION: received tool result id=%s err=%v", tr.result.ToolCallID, tr.err)
			if tr.err != nil {
				// Non-fatal: the server may have timed out or moved on.
				// The SSE stream will tell us what happens next.
				display.RenderWarning(fmt.Sprintf("failed to post tool result: %v", tr.err))
			}

		case input := <-inputCh:
			if input == "\x04" {
				return nil // Ctrl+D
			}
			if input == "" {
				continue // Ctrl+C cleared input
			}
			if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
				return nil
			}
			display.RenderUserMessage(input)
			msg := UserMessageEvent{
				Type:    "user_message",
				Content: input,
			}
			if err := client.PostUserInput(ctx, taskID, msg); err != nil {
				return fmt.Errorf("posting user input: %w", err)
			}
			display.StartThinking()
			agentIdle = false
			ctrlCPending = false

		case event, ok := <-events:
			if !ok {
				return nil // Stream ended
			}

			parsed := display.RenderEvent(event)
			if parsed == nil {
				continue
			}

			switch parsed.Type {
			case "exec_tool_call":
				// Execute tools concurrently in goroutines, but send results
				// through toolExecCh so the poster serializes the HTTP posts.
				display.LogDebug("SESSION: exec_tool_call tool=%s id=%s", parsed.ToolName, parsed.ToolCallID)
				go func() {
					var result ToolResponseEvent
					if executor.CanExecute(parsed.ToolName) {
						result = executor.Execute(ctx, parsed.ToolCallID, parsed.ToolName, parsed.ToolArgs)
					} else {
						result = ToolResponseEvent{
							Type:       "tool_response",
							ToolCallID: parsed.ToolCallID,
							Name:       parsed.ToolName,
							Content:    fmt.Sprintf("Error: tool %q is not available on this client", parsed.ToolName),
							IsError:    true,
						}
					}
					display.LogDebug("SESSION: tool execution complete id=%s err=%v", result.ToolCallID, result.IsError)
					select {
					case toolExecCh <- result:
					case <-ctx.Done():
					}
				}()

			case "user_approval_request":
				// Approval is handled via the input bar (RequestApproval) for
				// the executor's approvalFn. For SSE-delivered approval requests,
				// dispatch asynchronously so we don't block the event loop.
				go func() {
					approved := handleApproval(display, parsed, approvalMode)
					confirmation := UserConfirmationEvent{
						Type:       "user_confirmation",
						ApprovalID: parsed.ApprovalID,
						Approved:   approved,
					}
					if err := client.PostUserInput(ctx, taskID, confirmation); err != nil {
						// Best-effort; the error will surface on the next iteration.
						_ = err
					}
				}()

			case "assistant_message":
				display.LogDebug("SESSION: assistant_message is_final=%v len=%d", parsed.IsFinal, len(parsed.Message))
				if parsed.IsFinal {
					if !interactive {
						return nil // One-shot mode: done after final message
					}
					// Mark the agent as idle so the next select iteration
					// picks up input from the input bar channel.
					agentIdle = true
				}

			case "error":
				if !interactive {
					return fmt.Errorf("agent error: %s", parsed.Message)
				}
				// In interactive mode, go idle so the user can retry.
				agentIdle = true

			case "task_idle":
				// The server reports the task is idle but we didn't receive a
				// final assistant_message. This can happen if the agent crashes,
				// times out, or the error event was missed. Transition to idle
				// so the user can retry.
				if !agentIdle {
					display.LogDebug("SESSION: task_idle received while not idle, setting agentIdle=true")
					agentIdle = true
				}

			case "cancelled":
				return nil
			}
		}
	}
}


// handleApproval decides whether to approve a tool call based on the approval mode.
func handleApproval(display *Display, parsed *ParsedEvent, approvalMode *string) bool {
	switch *approvalMode {
	case "auto":
		return true
	case "manual":
		// Manual mode: always prompt the user for every operation.
		approved, err := display.RequestApproval(parsed.Message)
		if err != nil {
			return false
		}
		return approved
	case "balanced":
		// Auto-approve low sensitivity operations; prompt for high/destructive.
		if parsed.Sensitivity == "high" || parsed.Sensitivity == "destructive" {
			approved, err := display.RequestApproval(parsed.Message)
			if err != nil {
				return false
			}
			return approved
		}
		return true
	default:
		// Shouldn't happen due to validation, but prompt to be safe.
		approved, err := display.RequestApproval(parsed.Message)
		if err != nil {
			return false
		}
		return approved
	}
}
