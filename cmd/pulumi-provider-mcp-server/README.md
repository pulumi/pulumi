# Pulumi Provider MCP Server

A Model Context Protocol (MCP) server that provides direct access to Pulumi provider CRUD operations, bypassing the Pulumi engine for ad-hoc infrastructure management.

## Overview

This MCP server enables LLMs and AI agents to interact directly with Pulumi providers through a standardized interface. Unlike the full Pulumi engine, this server:

- Operates **session-based** with isolated provider registries
- Requires **explicit provider configuration** before use
- Provides **direct provider access** without state management
- Supports **multiple configurations** of the same provider (e.g., multi-region scenarios)

## Architecture

### Session-Based Design

Each MCP session maintains its own `providers.Registry` instance, ensuring complete isolation between different clients. Providers must be explicitly configured via `configure_provider` before they can be used.

### Key Components

- **Session State**: Tracks configured providers and caches schemas
- **Plugin Host**: Minimal implementation for provider loading and diagnostic routing
- **Serialization**: Bidirectional JSON/PropertyMap conversion with support for Pulumi-specific types (secrets, assets, archives)
- **Diagnostics**: Provider logs and status messages are routed as MCP notifications

## MCP Tools

The server exposes 11 MCP tools organized into four categories:

### Provider Lifecycle (1 tool)

#### `configure_provider`

Load and configure a provider instance.

**Parameters:**
- `package` (string, required): Provider package name (e.g., "aws", "kubernetes")
- `version` (string, optional): Semantic version (defaults to latest)
- `config` (object, optional): Provider configuration properties
- `id` (string, optional): User-supplied provider ID (auto-generated if not provided)

**Returns:**
- `providerId` (string): The provider ID to use in subsequent operations

**Example:**
```json
{
  "package": "aws",
  "version": "6.0.0",
  "config": {
    "region": "us-west-2"
  },
  "id": "aws-west"
}
```

### Introspection (3 tools)

#### `get_schema`

Retrieve complete provider schema for introspection.

**Parameters:**
- `providerId` (string, required): Configured provider ID

**Returns:**
- `schema` (object): Complete Pulumi package schema

#### `get_resource_schema`

Retrieve schema for a specific resource type.

**Parameters:**
- `providerId` (string, required): Configured provider ID
- `type` (string, required): Resource type token (e.g., "aws:s3/bucket:Bucket")

**Returns:**
- `resourceSchema` (object): Resource schema including input/output properties

#### `get_function_schema`

Retrieve schema for a specific function/invoke.

**Parameters:**
- `providerId` (string, required): Configured provider ID
- `token` (string, required): Function token (e.g., "aws:s3/getBucket:getBucket")

**Returns:**
- `functionSchema` (object): Function schema including inputs and outputs

### Resource Operations (6 tools)

#### `check`

Validate resource inputs against provider schema.

**Parameters:**
- `providerId` (string, required)
- `urn` (string, required): Resource URN
- `type` (string, required): Resource type token
- `inputs` (object, optional): Resource inputs to validate
- `oldInputs` (object, optional): Previous inputs for update scenarios
- `randomSeed` (string, optional): Base64-encoded random seed

**Returns:**
- `inputs` (object): Validated and normalized inputs
- `failures` (array): Validation failures if any

#### `diff`

Compare old and new resource properties to determine changes.

**Parameters:**
- `providerId` (string, required)
- `urn` (string, required)
- `id` (string, required): Resource ID
- `type` (string, required)
- `oldInputs` (object, required)
- `oldOutputs` (object, required)
- `newInputs` (object, required)

**Returns:**
- `changes` (string): "DIFF_NONE" or "DIFF_SOME"
- `replaces` (array): Properties that require replacement
- `deleteBeforeReplace` (boolean): Whether to delete before replacing
- `detailedDiff` (object): Detailed property-level diff information

#### `create`

Provision a new resource.

**Parameters:**
- `providerId` (string, required)
- `urn` (string, required)
- `type` (string, required)
- `inputs` (object, optional): Resource inputs
- `timeout` (number, optional): Timeout in seconds (default: 300)
- `preview` (boolean, optional): Preview mode (default: false)

**Returns:**
- `id` (string): Resource ID
- `properties` (object): Resource outputs

#### `read`

Read current live state of a resource.

**Parameters:**
- `providerId` (string, required)
- `urn` (string, required)
- `id` (string, required)
- `type` (string, required)
- `inputs` (object, optional): Last known inputs
- `properties` (object, optional): Last known outputs

**Returns:**
- `id` (string): Resource ID
- `properties` (object): Current resource state
- `inputs` (object): Current inputs

#### `update`

Update an existing resource.

**Parameters:**
- `providerId` (string, required)
- `urn` (string, required)
- `id` (string, required)
- `type` (string, required)
- `oldInputs` (object, required)
- `oldOutputs` (object, required)
- `newInputs` (object, required)
- `timeout` (number, optional): Timeout in seconds (default: 300)
- `preview` (boolean, optional): Preview mode (default: false)

**Returns:**
- `properties` (object): Updated resource outputs

#### `delete`

Deprovision an existing resource.

**Parameters:**
- `providerId` (string, required)
- `urn` (string, required)
- `id` (string, required)
- `type` (string, required)
- `properties` (object, required): Current resource state
- `timeout` (number, optional): Timeout in seconds (default: 300)

**Returns:**
- Empty object on success

### Function Operations (1 tool)

#### `invoke`

Execute a provider function (data source or utility function).

**Parameters:**
- `providerId` (string, required)
- `token` (string, required): Function token
- `args` (object, optional): Function arguments

**Returns:**
- `return` (object): Function return value
- `failures` (array): Validation failures if any

## Usage Example

Here's a complete example of creating an S3 bucket using the MCP server:

```javascript
// 1. Configure the AWS provider
const configResult = await callTool("configure_provider", {
  package: "aws",
  version: "6.0.0",
  config: {
    region: "us-west-2"
  },
  id: "aws-west"
});
const providerId = configResult.providerId; // "aws-west"

// 2. Get the bucket resource schema (optional, for introspection)
const schemaResult = await callTool("get_resource_schema", {
  providerId: "aws-west",
  type: "aws:s3/bucket:Bucket"
});

// 3. Check the inputs
const checkResult = await callTool("check", {
  providerId: "aws-west",
  urn: "urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::my-bucket",
  type: "aws:s3/bucket:Bucket",
  inputs: {
    bucket: "my-unique-bucket-name",
    acl: "private"
  }
});

// 4. Create the bucket
const createResult = await callTool("create", {
  providerId: "aws-west",
  urn: "urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::my-bucket",
  type: "aws:s3/bucket:Bucket",
  inputs: checkResult.inputs
});

console.log("Bucket ID:", createResult.id);
console.log("Bucket ARN:", createResult.properties.arn);

// 5. Update the bucket
const updateResult = await callTool("update", {
  providerId: "aws-west",
  urn: "urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::my-bucket",
  id: createResult.id,
  type: "aws:s3/bucket:Bucket",
  oldInputs: checkResult.inputs,
  oldOutputs: createResult.properties,
  newInputs: {
    bucket: "my-unique-bucket-name",
    acl: "public-read",
    tags: { Environment: "dev" }
  }
});

// 6. Delete the bucket
await callTool("delete", {
  providerId: "aws-west",
  urn: "urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::my-bucket",
  id: createResult.id,
  type: "aws:s3/bucket:Bucket",
  properties: updateResult.properties
});
```

## Multi-Region Example

The server supports multiple configurations of the same provider:

```javascript
// Configure AWS for us-west-2
const westProvider = await callTool("configure_provider", {
  package: "aws",
  config: { region: "us-west-2" },
  id: "aws-west"
});

// Configure AWS for us-east-1
const eastProvider = await callTool("configure_provider", {
  package: "aws",
  config: { region: "us-east-1" },
  id: "aws-east"
});

// Create bucket in us-west-2
const westBucket = await callTool("create", {
  providerId: "aws-west",
  urn: "urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::west-bucket",
  type: "aws:s3/bucket:Bucket",
  inputs: { bucket: "west-bucket" }
});

// Create bucket in us-east-1
const eastBucket = await callTool("create", {
  providerId: "aws-east",
  urn: "urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::east-bucket",
  type: "aws:s3/bucket:Bucket",
  inputs: { bucket: "east-bucket" }
});
```

## Special Property Types

The server handles Pulumi-specific property types using JSON encoding:

### Secrets

```json
{
  "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
  "value": "my-secret-value"
}
```

### Assets

Text asset:
```json
{
  "4dabf18193072939515e22adb298388d": "0def7320c3a5731c473e5ecbe6d01bc7",
  "text": "file contents"
}
```

Path asset:
```json
{
  "4dabf18193072939515e22adb298388d": "0def7320c3a5731c473e5ecbe6d01bc7",
  "path": "/path/to/file"
}
```

### Archives

Path archive:
```json
{
  "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
  "path": "/path/to/archive.zip"
}
```

### Resource References

```json
{
  "4dabf18193072939515e22adb298388d": "cfe97e649c90f5f7c0d6c9c3b0c4e3e6",
  "urn": "urn:pulumi:...",
  "id": "resource-id"
}
```

## Building

From the repository root:

```bash
make bin/pulumi-provider-mcp-server
```

The binary will be created at `bin/pulumi-provider-mcp-server`.

## Running

The server uses stdio transport (standard input/output) for MCP communication:

```bash
./bin/pulumi-provider-mcp-server
```

The server will wait for MCP messages on stdin and write responses to stdout. Diagnostic messages are logged to stderr.

## Development

### Project Structure

- `main.go`: MCP server setup and tool registration
- `session.go`: Session state management and provider lifecycle
- `host.go`: Plugin host implementation for provider loading
- `tools.go`: MCP tool handlers for all operations
- `schema.go`: Schema extraction utilities
- `serialization.go`: JSON/PropertyMap conversion utilities

### Testing

Run the tests:

```bash
cd cmd/pulumi-provider-mcp-server
go test ./...
```

### Linting

```bash
make lint
```

## Limitations

- No state persistence: This server does not maintain resource state between sessions
- No dependency tracking: Resources are managed independently
- No preview/dry-run at the engine level: Use `preview: true` on individual operations
- Requires explicit provider configuration before use

## License

Copyright 2016-2024, Pulumi Corporation.

Licensed under the Apache License, Version 2.0.
