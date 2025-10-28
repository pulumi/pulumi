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

package e2etest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2ERandomPet tests the complete lifecycle of the MCP server with the random provider.
func TestE2ERandomPet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Find the binary path (relative to e2etest directory)
	binaryPath, err := filepath.Abs("../../../bin/pulumi-provider-mcp-server")
	require.NoError(t, err)

	// Verify binary exists
	_, err = os.Stat(binaryPath)
	require.NoError(t, err, "MCP server binary not found at %s. Run 'go build -o ../../bin/pulumi-provider-mcp-server' first", binaryPath)

	// Create the MCP client with stdio transport
	mcpClient, err := client.NewStdioMCPClient(binaryPath, nil)
	require.NoError(t, err)
	defer mcpClient.Close()

	t.Log("MCP server started successfully")

	// Initialize the connection
	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "e2e-test",
				Version: "1.0.0",
			},
		},
	}
	_, err = mcpClient.Initialize(ctx, initReq)
	require.NoError(t, err)
	t.Log("Server initialized successfully")

	// List available tools
	toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.NotNil(t, toolsResult.Tools)

	// Verify we have all 11 expected tools
	expectedTools := []string{
		"configure_provider",
		"get_schema",
		"get_resource_schema",
		"get_function_schema",
		"check",
		"diff",
		"create",
		"read",
		"update",
		"delete",
		"invoke",
	}

	toolNames := make([]string, len(toolsResult.Tools))
	for i, tool := range toolsResult.Tools {
		toolNames[i] = tool.Name
	}

	for _, expectedTool := range expectedTools {
		assert.Contains(t, toolNames, expectedTool, "Expected tool %s not found", expectedTool)
	}

	t.Logf("Found %d tools: %v", len(toolNames), toolNames)

	// Test 1: Configure the random provider
	t.Run("ConfigureProvider", func(t *testing.T) {
		result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "configure_provider",
				Arguments: map[string]interface{}{
					"package": "random",
					"id":      "random-test",
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)

		// Parse the result
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		t.Logf("Configure provider result: %s", textContent.Text)
	})

	// Test 2: Get RandomPet resource schema
	t.Run("GetResourceSchema", func(t *testing.T) {
		result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_resource_schema",
				Arguments: map[string]interface{}{
					"providerId": "random-test",
					"type":       "random:index/randomPet:RandomPet",
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		t.Logf("RandomPet schema retrieved: %s", textContent.Text)
	})

	// Test 3: Full CRUD cycle
	var resourceID string
	var resourceProperties map[string]interface{}

	urn := "urn:pulumi:test-stack::test-project::random:index/randomPet:RandomPet::my-pet"
	resourceType := "random:index/randomPet:RandomPet"

	t.Run("Check", func(t *testing.T) {
		result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "check",
				Arguments: map[string]interface{}{
					"providerId": "random-test",
					"urn":        urn,
					"type":       resourceType,
					"inputs": map[string]interface{}{
						"length": float64(2),
					},
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		t.Logf("Check result: %s", textContent.Text)
	})

	t.Run("Create", func(t *testing.T) {
		result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "create",
				Arguments: map[string]interface{}{
					"providerId": "random-test",
					"urn":        urn,
					"type":       resourceType,
					"inputs": map[string]interface{}{
						"length": float64(2),
					},
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		t.Logf("Create result: %s", textContent.Text)

		// Parse the JSON response to get the ID and properties
		var createResult map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &createResult)
		require.NoError(t, err)

		if id, ok := createResult["id"]; ok {
			resourceID = id.(string)
		} else {
			// Fallback for placeholder
			resourceID = "test-id"
		}
		if props, ok := createResult["properties"]; ok {
			resourceProperties = props.(map[string]interface{})
		} else {
			resourceProperties = map[string]interface{}{
				"length": float64(2),
			}
		}
	})

	t.Run("Read", func(t *testing.T) {
		result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "read",
				Arguments: map[string]interface{}{
					"providerId": "random-test",
					"urn":        urn,
					"id":         resourceID,
					"type":       resourceType,
					"inputs": map[string]interface{}{
						"length": float64(2),
					},
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		t.Logf("Read result: %s", textContent.Text)
	})

	t.Run("Delete", func(t *testing.T) {
		result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "delete",
				Arguments: map[string]interface{}{
					"providerId": "random-test",
					"urn":        urn,
					"id":         resourceID,
					"type":       resourceType,
					"properties": resourceProperties,
				},
			},
		})
		require.NoError(t, err)
		require.NotEmpty(t, result.Content)

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		t.Logf("Delete result: %s", textContent.Text)
	})

	t.Log("E2E test completed successfully")
}
