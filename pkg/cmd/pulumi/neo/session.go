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

// getBackendInfo extracts the API URL, access token, and organization from the
// current Pulumi Cloud credentials. The orgFlag overrides automatic org resolution.
func getBackendInfo(ctx context.Context, orgFlag string) (apiURL, apiToken, org string, err error) {
	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return "", "", "", fmt.Errorf("not logged in: %w", err)
	}

	apiURL = creds.Current
	if apiURL == "" {
		return "", "", "", fmt.Errorf("no current backend, please run 'pulumi login'")
	}

	apiToken = creds.AccessTokens[apiURL]
	if apiToken == "" {
		return "", "", "", fmt.Errorf("no access token for %s, please run 'pulumi login'", apiURL)
	}

	if orgFlag != "" {
		org = orgFlag
	} else {
		// Try to resolve the org from the stored account info.
		if account, ok := creds.Accounts[apiURL]; ok && account.Username != "" {
			org = account.Username
		}
	}

	if org == "" {
		return "", "", "", fmt.Errorf("could not determine organization; use --org to specify one")
	}

	return apiURL, apiToken, org, nil
}

// runNeo is the main entry point for `pulumi neo [prompt]`.
// It creates a new task on the service and begins the event loop.
func runNeo(ctx context.Context, prompt string, orgFlag string, interactive bool, jsonOutput bool,
	approvalMode string,
) error {
	if err := validateApprovalMode(approvalMode); err != nil {
		return err
	}

	apiURL, apiToken, org, err := getBackendInfo(ctx, orgFlag)
	if err != nil {
		return err
	}

	client := NewNeoClient(apiURL, apiToken, org)
	display := NewDisplay(os.Stdout, os.Stderr, os.Stdin, interactive, jsonOutput)

	clientMode := "cli"
	if !interactive {
		clientMode = "cli-noninteractive"
	}

	if prompt == "" && interactive {
		// Interactive mode without an initial prompt - just create a session
		// and wait for the user to type. For now, require a prompt.
		return fmt.Errorf("interactive mode is not yet supported; please provide a prompt")
	}

	taskID, err := client.CreateTask(ctx, prompt, clientMode)
	if err != nil {
		return err
	}

	if !jsonOutput {
		display.RenderSessionStart(taskID)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	executor := NewToolExecutor(workDir, func(req ApprovalRequest) (bool, error) {
		return display.PromptApproval(req.Message)
	})

	return runEventLoop(ctx, client, taskID, executor, display, interactive, approvalMode)
}

// runAttach attaches to an existing Neo session by task ID.
func runAttach(ctx context.Context, taskID string, orgFlag string, interactive bool, jsonOutput bool,
	approvalMode string,
) error {
	if err := validateApprovalMode(approvalMode); err != nil {
		return err
	}

	apiURL, apiToken, org, err := getBackendInfo(ctx, orgFlag)
	if err != nil {
		return err
	}

	client := NewNeoClient(apiURL, apiToken, org)
	display := NewDisplay(os.Stdout, os.Stderr, os.Stdin, interactive, jsonOutput)

	// Update the task's client mode to indicate a CLI client is now attached.
	clientMode := "cli"
	if !interactive {
		clientMode = "cli-noninteractive"
	}
	if err := client.UpdateTask(ctx, taskID, clientMode); err != nil {
		return fmt.Errorf("attaching to session: %w", err)
	}

	if !jsonOutput {
		display.RenderSessionAttach(taskID)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	executor := NewToolExecutor(workDir, func(req ApprovalRequest) (bool, error) {
		return display.PromptApproval(req.Message)
	})

	return runEventLoop(ctx, client, taskID, executor, display, interactive, approvalMode)
}

// runListSessions lists recent Neo sessions for the organization.
func runListSessions(ctx context.Context, orgFlag string) error {
	apiURL, apiToken, org, err := getBackendInfo(ctx, orgFlag)
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

// runEventLoop connects to the SSE stream and processes events, dispatching
// tool calls to the executor, rendering output via the display, and handling
// user interaction for approvals and multi-turn conversation.
func runEventLoop(ctx context.Context, client *NeoClient, taskID string,
	executor *ToolExecutor, display *Display, interactive bool, approvalMode string,
) error {
	events, errCh := client.StreamEvents(ctx, taskID, "")

	// Tool execution runs asynchronously so the event loop continues draining SSE
	// events (heartbeats, progress) while a tool is executing. Only one tool
	// runs at a time since the agent waits for each tool result before continuing.
	toolResultCh := make(chan toolResult, 1)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("stream error: %w", err)
			}

		case tr := <-toolResultCh:
			if tr.err != nil {
				return fmt.Errorf("posting tool result: %w", tr.err)
			}

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
				// Execute tool asynchronously so the event loop keeps draining SSE events.
				go func() {
					result := executor.Execute(ctx, parsed.ToolCallID, parsed.ToolName, parsed.ToolArgs)
					postErr := client.PostToolResponse(ctx, taskID, result)
					toolResultCh <- toolResult{result: result, err: postErr}
				}()

			case "user_approval_request":
				approved := handleApproval(display, parsed, approvalMode)
				confirmation := UserConfirmationEvent{
					Type:       "user_confirmation",
					ApprovalID: parsed.ApprovalID,
					Approved:   approved,
				}
				if err := client.PostUserInput(ctx, taskID, confirmation); err != nil {
					return fmt.Errorf("posting approval: %w", err)
				}

			case "assistant_message":
				if parsed.IsFinal {
					if !interactive {
						return nil // One-shot mode: done after final message
					}
					// Interactive: prompt for next user input.
					input, err := display.PromptUserInput()
					if err != nil {
						return err
					}
					if input == "" {
						continue
					}
					if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
						return nil
					}
					msg := UserMessageEvent{
						Type:    "user_message",
						Content: input,
					}
					if err := client.PostUserInput(ctx, taskID, msg); err != nil {
						return fmt.Errorf("posting user input: %w", err)
					}
				}

			case "error":
				if !interactive {
					return fmt.Errorf("agent error: %s", parsed.Message)
				}

			case "cancelled":
				return nil
			}
		}
	}
}

// handleApproval decides whether to approve a tool call based on the approval mode.
func handleApproval(display *Display, parsed *ParsedEvent, approvalMode string) bool {
	switch approvalMode {
	case "auto":
		return true
	case "manual":
		// Manual mode: always prompt the user for every operation.
		approved, err := display.PromptApproval(parsed.Message)
		if err != nil {
			return false
		}
		return approved
	case "balanced":
		// Auto-approve low sensitivity operations; prompt for high/destructive.
		if parsed.Sensitivity == "high" || parsed.Sensitivity == "destructive" {
			approved, err := display.PromptApproval(parsed.Message)
			if err != nil {
				return false
			}
			return approved
		}
		return true
	default:
		// Shouldn't happen due to validation, but prompt to be safe.
		approved, err := display.PromptApproval(parsed.Message)
		if err != nil {
			return false
		}
		return approved
	}
}
