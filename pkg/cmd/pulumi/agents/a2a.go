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

package agents

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//go:embed skill/SKILL.md
var delegationSkill string

const approvalExtension = "urn:pulumi:a2a:approval:v1"

type rawCaller interface {
	RawCallWithoutRedirects(
		context.Context, string, string, url.Values, io.Reader, http.Header, bool,
	) (*http.Response, error)
}

type a2aClient struct {
	caller   rawCaller
	cloudURL string
}

type agentCard struct {
	SupportedInterfaces []struct {
		URL             string `json:"url"`
		ProtocolBinding string `json:"protocolBinding"`
		ProtocolVersion string `json:"protocolVersion"`
	} `json:"supportedInterfaces"`
	Capabilities struct {
		Streaming bool `json:"streaming"`
	} `json:"capabilities"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if len(e.Data) > 0 {
		return fmt.Sprintf("A2A error %d: %s (%s)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("A2A error %d: %s", e.Code, e.Message)
}

func currentClient(ctx context.Context) (*a2aClient, error) {
	project, _, err := pkgWorkspace.Instance.ReadProject("")
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}
	backend, err := cmdBackend.NonInteractiveCurrentBackend(
		ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, project,
	)
	if err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, errors.New("log in with pulumi login before using Pulumi Agents")
	}
	cloud, ok := backend.(httpstate.Backend)
	if !ok {
		return nil, errors.New("Pulumi Agents requires a Pulumi Cloud backend; the active backend is an OSS backend")
	}
	return &a2aClient{caller: cloud.Client(), cloudURL: cloud.CloudURL()}, nil
}

func (c *a2aClient) request(
	ctx context.Context, method string, path string, body []byte, streaming bool,
) (*http.Response, error) {
	headers := http.Header{"A2A-Version": {"1.0"}, "A2A-Extensions": {approvalExtension}, "Accept": {"application/json"}}
	if body != nil {
		headers.Set("Content-Type", "application/json")
	}
	if streaming {
		headers.Set("Accept", "text/event-stream")
	}
	response, err := c.caller.RawCallWithoutRedirects(ctx, method, path, nil, bytes.NewReader(body), headers, false)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		if response.StatusCode == http.StatusUnauthorized {
			return nil, errors.New("Pulumi authentication failed; use pulumi login to refresh the active login")
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Pulumi A2A request failed (HTTP %d): %s",
			response.StatusCode, strings.TrimSpace(string(data)))
	}
	return response, nil
}

func catalogPath(org string) string { return "/api/preview/agents/" + url.PathEscape(org) + "/a2a" }

func (c *a2aClient) card(ctx context.Context, org string, agent string) ([]byte, agentCard, error) {
	path := catalogPath(org) + "/" + url.PathEscape(agent) + "/.well-known/agent-card.json"
	response, err := c.request(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return nil, agentCard{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return nil, agentCard{}, err
	}
	var card agentCard
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, card, fmt.Errorf("invalid Agent Card: %w", err)
	}
	return data, card, nil
}

func (c *a2aClient) endpoint(card agentCard) (string, error) {
	base, err := url.Parse(c.cloudURL)
	if err != nil {
		return "", err
	}
	for _, entry := range card.SupportedInterfaces {
		if entry.ProtocolBinding != "JSONRPC" || entry.ProtocolVersion != "1.0" {
			continue
		}
		u, err := url.Parse(entry.URL)
		if err != nil || u.User != nil || u.Fragment != "" || u.RawQuery != "" ||
			u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
			return "", errors.New("Agent Card endpoint must use the active Pulumi backend origin")
		}
		local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
		if u.Scheme != "https" && (u.Scheme != "http" || !local) {
			return "", errors.New("Agent Card endpoint must use HTTPS")
		}
		segments := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(segments) != 6 || segments[0] != "api" || segments[1] != "preview" ||
			segments[2] != "agents" || segments[4] != "a2a" ||
			segments[3] == "" || segments[3] == "." || segments[3] == ".." ||
			segments[5] == "" || segments[5] == "." || segments[5] == ".." {
			return "", errors.New("Agent Card endpoint is outside the Pulumi Agents API")
		}
		return u.EscapedPath(), nil
	}
	return "", errors.New("Agent Card does not advertise an A2A 1.0 JSON-RPC interface")
}

func writeJSON(out io.Writer, data []byte, compact bool) error {
	var buf bytes.Buffer
	var err error
	if compact {
		err = json.Compact(&buf, data)
	} else {
		err = json.Indent(&buf, data, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}
	_, err = fmt.Fprintln(out, buf.String())
	return err
}

func unwrapRPC(data []byte) (json.RawMessage, error) {
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC response: %w", err)
	}
	if response.JSONRPC != "2.0" || string(response.ID) != "1" {
		return nil, errors.New("invalid JSON-RPC version or response ID")
	}
	if response.Error != nil {
		return nil, response.Error
	}
	if len(response.Result) == 0 {
		return nil, errors.New("JSON-RPC response has no result")
	}
	return response.Result, nil
}

func readSSE(body io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var data []string
	ended := false
	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		result, err := unwrapRPC([]byte(strings.Join(data, "\n")))
		data = nil
		if err != nil {
			return err
		}
		var event struct {
			Task *struct {
				Status struct {
					State string `json:"state"`
				} `json:"status"`
			} `json:"task"`
			StatusUpdate *struct {
				Status struct {
					State string `json:"state"`
				} `json:"status"`
			} `json:"statusUpdate"`
			Message json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(result, &event); err != nil {
			return err
		}
		state := ""
		if event.Task != nil {
			state = event.Task.Status.State
		}
		if event.StatusUpdate != nil {
			state = event.StatusUpdate.Status.State
		}
		switch state {
		case "TASK_STATE_COMPLETED", "TASK_STATE_FAILED", "TASK_STATE_CANCELED", "TASK_STATE_REJECTED",
			"TASK_STATE_INPUT_REQUIRED", "TASK_STATE_AUTH_REQUIRED":
			ended = true
		}
		ended = ended || len(event.Message) > 0
		return writeJSON(out, result, true)
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(value, " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	if !ended {
		return errors.New("A2A stream ended before a terminal or interrupted state; use get or watch to reconnect")
	}
	return nil
}

// NewAgentsCmd creates commands for discovering and interacting with hosted Pulumi Agents.
func NewAgentsCmd() *cobra.Command {
	return newAgentsCmd(currentClient)
}

func newAgentsCmd(connect func(context.Context) (*a2aClient, error)) *cobra.Command {
	root := &cobra.Command{Use: "agents", Short: "Discover and interact with Pulumi Agents"}
	group := &cobra.Command{Use: "a2a", Short: "Delegate work using the A2A 1.0 protocol"}
	root.AddCommand(group)
	var org string
	var compact bool
	group.PersistentFlags().StringVar(&org, "org", "", "Pulumi organization")
	group.PersistentFlags().BoolVar(&compact, "json", false, "Emit compact A2A JSON results")

	group.AddCommand(&cobra.Command{
		Use: "skill", Short: "Print delegation instructions for Codex and Claude Code", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), delegationSkill)
			return err
		},
	})
	discover := &cobra.Command{
		Use: "discover", Short: "List the agents available to the active Pulumi identity", Args: cobra.NoArgs,
	}
	discover.RunE = func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(org) == "" {
			return errors.New("--org is required")
		}
		client, err := connect(cmd.Context())
		if err != nil {
			return err
		}
		response, err := client.request(cmd.Context(), http.MethodGet, catalogPath(org), nil, false)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
		if err != nil {
			return err
		}
		return writeJSON(cmd.OutOrStdout(), data, compact)
	}
	cardCmd := &cobra.Command{Use: "card <agent>", Short: "Get an authenticated Agent Card", Args: cobra.ExactArgs(1)}
	cardCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(org) == "" {
			return errors.New("--org is required")
		}
		client, err := connect(cmd.Context())
		if err != nil {
			return err
		}
		data, _, err := client.card(cmd.Context(), org, args[0])
		if err != nil {
			return err
		}
		return writeJSON(cmd.OutOrStdout(), data, compact)
	}
	group.AddCommand(discover, cardCmd)
	for _, operation := range []struct {
		name   string
		use    string
		method string
		help   string
	}{
		{"send", "send <agent> --request <file>", "SendMessage", "Send an A2A message to a Pulumi Agent"},
		{"get", "get <task> --agent <agent>", "GetTask", "Get an A2A task"},
		{"list", "list --agent <agent>", "ListTasks", "List A2A tasks for an agent"},
		{"watch", "watch <task> --agent <agent>", "SubscribeToTask", "Subscribe to an A2A task"},
		{"cancel", "cancel <task> --agent <agent>", "CancelTask", "Request cancellation of an A2A task"},
	} {
		var agent string
		var requestFile string
		var streaming bool
		var immediately bool
		var historyLength int
		var pageSize int
		var pageToken string
		var contextID string
		var status string
		var timestamp string
		var includeArtifacts bool
		cmd := &cobra.Command{Use: operation.use, Short: operation.help, Args: cobra.ExactArgs(1)}
		if operation.name == "list" {
			cmd.Args = cobra.NoArgs
		}
		if operation.name == "send" {
			cmd.Flags().StringVar(&requestFile, "request", "", "A2A SendMessageRequest JSON file, or - for stdin")
			cmd.Flags().BoolVar(&streaming, "stream", false, "Use SendStreamingMessage and emit one StreamResponse per line")
			cmd.Flags().BoolVar(&immediately, "return-immediately", false, "Set configuration.returnImmediately to true")
		} else {
			cmd.Flags().StringVar(&agent, "agent", "", "Agent identity from discover")
		}
		if operation.name == "get" || operation.name == "list" {
			cmd.Flags().IntVar(&historyLength, "history-length", 0, "Limit message history; omit for all history")
		}
		if operation.name == "list" {
			cmd.Flags().IntVar(&pageSize, "page-size", 50, "Maximum number of tasks, from 1 to 100")
			cmd.Flags().StringVar(&pageToken, "page-token", "", "Continuation token from ListTasks")
			cmd.Flags().StringVar(&contextID, "context", "", "Filter by A2A contextId")
			cmd.Flags().StringVar(&status, "status", "", "Filter by A2A task state enum")
			cmd.Flags().StringVar(&timestamp, "status-timestamp-after", "", "Filter by status timestamp in RFC 3339 format")
			cmd.Flags().BoolVar(&includeArtifacts, "include-artifacts", false, "Include task artifacts")
		}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(org) == "" {
				return errors.New("--org is required")
			}
			params := map[string]any{}
			method := operation.method
			stream := streaming || operation.name == "watch"
			if operation.name == "send" {
				agent = args[0]
				if requestFile == "" {
					return errors.New("--request is required")
				}
				var data []byte
				var err error
				if requestFile == "-" {
					data, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1024*1024+1))
				} else {
					file, openErr := os.Open(requestFile)
					if openErr != nil {
						return openErr
					}
					defer file.Close()
					data, err = io.ReadAll(io.LimitReader(file, 1024*1024+1))
				}
				if err != nil {
					return err
				}
				if len(data) > 1024*1024 {
					return errors.New("request exceeds 1 MiB")
				}
				if err := json.Unmarshal(data, &params); err != nil || params == nil {
					return errors.New("--request must contain an A2A SendMessageRequest JSON object")
				}
				if cmd.Flags().Changed("return-immediately") {
					configuration, ok := params["configuration"].(map[string]any)
					if !ok {
						configuration = map[string]any{}
					}
					configuration["returnImmediately"] = immediately
					params["configuration"] = configuration
				}
				if stream {
					method = "SendStreamingMessage"
				}
			} else {
				if agent == "" {
					return errors.New("--agent is required")
				}
				if operation.name != "list" {
					params["id"] = args[0]
				}
				if cmd.Flags().Changed("history-length") {
					if historyLength < 0 {
						return errors.New("--history-length must be non-negative")
					}
					params["historyLength"] = historyLength
				}
			}
			if operation.name == "list" {
				if pageSize < 1 || pageSize > 100 {
					return errors.New("--page-size must be between 1 and 100")
				}
				params["pageSize"] = pageSize
				params["includeArtifacts"] = includeArtifacts
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				if contextID != "" {
					params["contextId"] = contextID
				}
				if status != "" {
					params["status"] = status
				}
				if timestamp != "" {
					params["statusTimestampAfter"] = timestamp
				}
			}
			client, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			_, card, err := client.card(cmd.Context(), org, agent)
			if err != nil {
				return err
			}
			if stream && !card.Capabilities.Streaming {
				return errors.New("agent does not support streaming")
			}
			endpoint, err := client.endpoint(card)
			if err != nil {
				return err
			}
			body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
			if err != nil {
				return err
			}
			response, err := client.request(cmd.Context(), http.MethodPost, endpoint, body, stream)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			if stream {
				mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
				if err == nil && mediaType == "text/event-stream" {
					return readSSE(response.Body, cmd.OutOrStdout())
				}
			}
			data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024))
			if err != nil {
				return err
			}
			result, err := unwrapRPC(data)
			if err != nil {
				return err
			}
			if stream {
				return errors.New("expected an A2A SSE response")
			}
			return writeJSON(cmd.OutOrStdout(), result, compact)
		}
		group.AddCommand(cmd)
	}
	return root
}
