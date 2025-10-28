// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

const (
	serverName    = "pulumi-provider-mcp-server"
	serverVersion = "0.1.0"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func run() error {
	// Create the MCP server
	s := server.NewMCPServer(serverName, serverVersion)

	// Session state
	var session *Session

	// Initialize session on first request
	initSession := func(ctx context.Context) error {
		if session != nil {
			return nil
		}

		// Create callbacks for diagnostic messages
		// For now, just log to stderr
		logCallback := func(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
			level := severityToLogLevel(sev)
			logMsg := fmt.Sprintf("[%s] %s", level, msg)
			if urn != "" {
				logMsg = fmt.Sprintf("[%s] [%s] %s", level, urn, msg)
			}
			log.Println(logMsg)
			// TODO: Send as MCP notification when API is figured out
		}

		logStatusCallback := func(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
			level := severityToLogLevel(sev)
			logMsg := fmt.Sprintf("[%s] [STATUS] %s", level, msg)
			if urn != "" {
				logMsg = fmt.Sprintf("[%s] [STATUS] [%s] %s", level, urn, msg)
			}
			log.Println(logMsg)
			// TODO: Send as MCP notification when API is figured out
		}

		var err error
		session, err = NewSession(ctx, logCallback, logStatusCallback)
		return err
	}

	// Register tools

	// Provider lifecycle
	s.AddTool(mcp.Tool{
		Name:        "configure_provider",
		Description: "Load and configure a provider instance",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"package": map[string]any{
					"type":        "string",
					"description": "Provider package name (required)",
				},
				"version": map[string]any{
					"type":        "string",
					"description": "Semantic version (optional, defaults to latest)",
				},
				"config": map[string]any{
					"type":        "object",
					"description": "Provider configuration (optional)",
				},
				"id": map[string]any{
					"type":        "string",
					"description": "User-supplied provider ID (optional, auto-generated if not provided)",
				},
			},
			Required: []string{"package"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := initSession(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to initialize session: %v", err)), nil
		}

		argsMap, err := getArgsMap(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := handleConfigureProvider(ctx, session, argsMap)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(formatJSON(result)), nil
	})

	// Provider introspection
	s.AddTool(mcp.Tool{
		Name:        "get_schema",
		Description: "Retrieve complete provider schema for introspection",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{
					"type":        "string",
					"description": "Configured provider ID (required)",
				},
			},
			Required: []string{"providerId"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := initSession(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to initialize session: %v", err)), nil
		}

		argsMap, err := getArgsMap(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := handleGetSchema(ctx, session, argsMap)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(formatJSON(result)), nil
	})

	s.AddTool(mcp.Tool{
		Name:        "get_resource_schema",
		Description: "Retrieve schema for a specific resource type",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{
					"type":        "string",
					"description": "Configured provider ID (required)",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "Resource type token (required)",
				},
			},
			Required: []string{"providerId", "type"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := initSession(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to initialize session: %v", err)), nil
		}

		argsMap, err := getArgsMap(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := handleGetResourceSchema(ctx, session, argsMap)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(formatJSON(result)), nil
	})

	s.AddTool(mcp.Tool{
		Name:        "get_function_schema",
		Description: "Retrieve schema for a specific invoke/function",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{
					"type":        "string",
					"description": "Configured provider ID (required)",
				},
				"token": map[string]any{
					"type":        "string",
					"description": "Function token (required)",
				},
			},
			Required: []string{"providerId", "token"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := initSession(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to initialize session: %v", err)), nil
		}

		argsMap, err := getArgsMap(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := handleGetFunctionSchema(ctx, session, argsMap)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(formatJSON(result)), nil
	})

	// Resource operations
	registerResourceTools(s, &session, initSession)

	// Function operations
	s.AddTool(mcp.Tool{
		Name:        "invoke",
		Description: "Execute a provider function (data source or utility function)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{
					"type":        "string",
					"description": "Configured provider ID (required)",
				},
				"token": map[string]any{
					"type":        "string",
					"description": "Function token (required)",
				},
				"args": map[string]any{
					"type":        "object",
					"description": "Function arguments (optional)",
				},
			},
			Required: []string{"providerId", "token"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := initSession(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to initialize session: %v", err)), nil
		}

		argsMap, err := getArgsMap(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := handleInvoke(ctx, session, argsMap)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(formatJSON(result)), nil
	})

	// Start server on stdio
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	// Cleanup
	if session != nil {
		_ = session.Close()
	}

	return nil
}

// registerResourceTools registers all resource operation tools.
func registerResourceTools(s *server.MCPServer, session **Session, initSession func(context.Context) error) {
	// Check tool
	s.AddTool(mcp.Tool{
		Name:        "check",
		Description: "Validate resource inputs against provider schema",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{"type": "string", "description": "Configured provider ID"},
				"urn":        map[string]any{"type": "string", "description": "Resource URN"},
				"type":       map[string]any{"type": "string", "description": "Resource type token"},
				"inputs":     map[string]any{"type": "object", "description": "Resource inputs"},
				"oldInputs":  map[string]any{"type": "object", "description": "Previous inputs (optional)"},
				"randomSeed": map[string]any{"type": "string", "description": "Base64-encoded random seed (optional)"},
			},
			Required: []string{"providerId", "urn", "type"},
		},
	}, makeToolHandler(session, initSession, handleCheck))

	// Diff tool
	s.AddTool(mcp.Tool{
		Name:        "diff",
		Description: "Compare old and new resource properties to determine changes",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{"type": "string"},
				"urn":        map[string]any{"type": "string"},
				"id":         map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"oldInputs":  map[string]any{"type": "object"},
				"oldOutputs": map[string]any{"type": "object"},
				"newInputs":  map[string]any{"type": "object"},
			},
			Required: []string{"providerId", "urn", "id", "type", "oldInputs", "oldOutputs", "newInputs"},
		},
	}, makeToolHandler(session, initSession, handleDiff))

	// Create tool
	s.AddTool(mcp.Tool{
		Name:        "create",
		Description: "Provision a new resource",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{"type": "string"},
				"urn":        map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"inputs":     map[string]any{"type": "object"},
				"timeout":    map[string]any{"type": "number", "description": "Timeout in seconds (optional)"},
				"preview":    map[string]any{"type": "boolean", "description": "Preview mode (optional)"},
			},
			Required: []string{"providerId", "urn", "type"},
		},
	}, makeToolHandler(session, initSession, handleCreate))

	// Read tool
	s.AddTool(mcp.Tool{
		Name:        "read",
		Description: "Read current live state of a resource",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{"type": "string"},
				"urn":        map[string]any{"type": "string"},
				"id":         map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"inputs":     map[string]any{"type": "object", "description": "Last known inputs (optional)"},
				"properties": map[string]any{"type": "object", "description": "Last known outputs (optional)"},
			},
			Required: []string{"providerId", "urn", "id", "type"},
		},
	}, makeToolHandler(session, initSession, handleRead))

	// Update tool
	s.AddTool(mcp.Tool{
		Name:        "update",
		Description: "Update an existing resource",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{"type": "string"},
				"urn":        map[string]any{"type": "string"},
				"id":         map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"oldInputs":  map[string]any{"type": "object"},
				"oldOutputs": map[string]any{"type": "object"},
				"newInputs":  map[string]any{"type": "object"},
				"timeout":    map[string]any{"type": "number", "description": "Timeout in seconds (optional)"},
				"preview":    map[string]any{"type": "boolean", "description": "Preview mode (optional)"},
			},
			Required: []string{"providerId", "urn", "id", "type", "oldInputs", "oldOutputs", "newInputs"},
		},
	}, makeToolHandler(session, initSession, handleUpdate))

	// Delete tool
	s.AddTool(mcp.Tool{
		Name:        "delete",
		Description: "Deprovision an existing resource",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providerId": map[string]any{"type": "string"},
				"urn":        map[string]any{"type": "string"},
				"id":         map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"properties": map[string]any{"type": "object"},
				"timeout":    map[string]any{"type": "number", "description": "Timeout in seconds (optional)"},
			},
			Required: []string{"providerId", "urn", "id", "type", "properties"},
		},
	}, makeToolHandler(session, initSession, handleDelete))
}

// makeToolHandler creates a tool handler that initializes session and calls the handler function.
func makeToolHandler(
	session **Session,
	initSession func(context.Context) error,
	handler func(context.Context, *Session, map[string]any) (map[string]any, error),
) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := initSession(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to initialize session: %v", err)), nil
		}

		argsMap, err := getArgsMap(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := handler(ctx, *session, argsMap)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(formatJSON(result)), nil
	}
}

// severityToLogLevel converts Pulumi severity to MCP log level.
func severityToLogLevel(sev diag.Severity) string {
	switch sev {
	case diag.Debug:
		return "debug"
	case diag.Info, diag.Infoerr:
		return "info"
	case diag.Warning:
		return "warning"
	case diag.Error:
		return "error"
	default:
		return "info"
	}
}

// formatJSON formats a map as JSON string.
func formatJSON(v any) string {
	// For now, use fmt.Sprintf - could use json.Marshal for better formatting
	return fmt.Sprintf("%v", v)
}

// getArgsMap converts request arguments to a map[string]any with type assertion.
func getArgsMap(args any) (map[string]any, error) {
	if args == nil {
		return map[string]any{}, nil
	}
	argsMap, ok := args.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("arguments must be a map, got %T", args)
	}
	return argsMap, nil
}
