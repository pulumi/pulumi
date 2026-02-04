# Pulumi CLI Agent-Friendly Improvements

## Executive Summary

This document outlines a comprehensive plan for improving the Pulumi CLI for agent/AI and automation usage. After extensive analysis of the codebase, several areas have been identified for improvement to make the CLI more suitable for programmatic interaction. The CLI already has foundational support (JSON output, non-interactive mode, environment variables), but there are significant gaps in consistency, error handling, and machine-readable interfaces.

---

## 1. Current State Analysis

### 1.1 Existing Agent-Friendly Features

#### JSON Output Support (Partial)
Commands currently supporting `--json`:
- `preview`, `up`, `destroy`, `refresh`
- `stack output`, `stack ls`, `stack history`, `stack tag ls`
- `config`
- `whoami`
- `plugin ls`
- `logs`
- `project ls`

Implementation pattern: Each command defines its own JSON struct (e.g., `whoAmIJSON`, `stackSummaryJSON`) in their respective files.

#### Non-Interactive Mode
- Global `--non-interactive` flag sets `cmdutil.DisableInteractive`
- CI detection via `ciutil.IsCI()` in `sdk/go/common/util/ciutil/`

#### Environment Variable Support
- All flags exposed as `OPTION_*` environment variables (auto-generated in `pulumi.go:declareFlagsAsEnvironmentVariables`)
- Pulumi-specific environment variables defined in `sdk/go/common/env/env.go`

#### CLI Specification Generation
- Hidden command `generate-cli-spec` outputs JSON schema of all commands
- Location: `pkg/cmd/pulumi/clispec/gen.go`

#### Event Streaming System
- Rich event types in `pkg/engine/events.go`
- Streaming JSON for operations via `ShowJSONEvents()`
- Event types: `CancelEvent`, `DiagEvent`, `PreludeEvent`, `SummaryEvent`, `ResourcePreEvent`, `ResourceOutputsEvent`, etc.

### 1.2 Key Pain Points for Agent Usage

| Issue | Impact | Current Behavior |
|-------|--------|------------------|
| Inconsistent JSON output | Agents cannot reliably parse all commands | Some commands lack `--json`, others have different formats |
| Human-readable errors | Difficult to programmatically handle errors | Errors go to stderr as plain text |
| Single exit code | Cannot distinguish error types | All errors return -1 |
| Preview vs streaming JSON | Two different JSON formats | `PULUMI_ENABLE_STREAMING_JSON_PREVIEW` changes behavior |
| Interactive prompts | Blocks automation | Some prompts cannot be skipped |
| No operation IDs | Cannot track/correlate operations | Operations don't return unique IDs |
| Mixed stdout/stderr | Hard to separate data from diagnostics | Progress, warnings, and data intermixed |

---

## 2. Proposed Improvements

### 2.1 Standardized JSON Output Layer

**Goal**: Every command should support `--json` with a consistent envelope structure.

#### Proposed JSON Envelope

```go
// Location: pkg/cmd/pulumi/ui/json_envelope.go (new file)

type JSONEnvelope struct {
    Version    string          `json:"version"`           // Schema version (e.g., "1.0")
    Command    string          `json:"command"`           // Command that was run
    Success    bool            `json:"success"`           // Whether the command succeeded
    Result     interface{}     `json:"result,omitempty"`  // Command-specific result data
    Error      *JSONError      `json:"error,omitempty"`   // Structured error if failed
    Warnings   []JSONDiag      `json:"warnings,omitempty"`// Any warnings
    Metadata   *JSONMetadata   `json:"metadata,omitempty"`// Operation metadata
}

type JSONError struct {
    Code       string   `json:"code"`              // Machine-readable error code
    Message    string   `json:"message"`           // Human-readable message
    Details    any      `json:"details,omitempty"` // Error-specific details
    Stack      string   `json:"stack,omitempty"`   // Stack trace (debug mode)
}

type JSONDiag struct {
    Severity string `json:"severity"`  // "warning", "info"
    Message  string `json:"message"`
    URN      string `json:"urn,omitempty"`
}

type JSONMetadata struct {
    OperationID   string    `json:"operationId,omitempty"`
    Duration      string    `json:"duration,omitempty"`
    Timestamp     time.Time `json:"timestamp"`
}
```

#### Example Output

```json
{
  "version": "1.0",
  "command": "stack init",
  "success": true,
  "result": {
    "name": "dev",
    "organization": "myorg",
    "url": "https://app.pulumi.com/myorg/myproject/dev"
  },
  "metadata": {
    "operationId": "op-abc123",
    "duration": "1.234s",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

#### Commands Needing JSON Support

| Command | Current Status | Proposed Result Schema |
|---------|----------------|------------------------|
| `pulumi new` | No JSON | `{name, path, template, runtime}` |
| `pulumi stack init` | No JSON | `{name, organization, url}` |
| `pulumi stack select` | No JSON | `{name, organization, url}` |
| `pulumi stack rm` | No JSON | `{name, deleted: true}` |
| `pulumi state *` | Partial | Various state-specific schemas |
| `pulumi login` | No JSON | `{user, organizations, backend}` |
| `pulumi logout` | No JSON | `{success: true}` |
| `pulumi cancel` | No JSON | `{updateId, cancelled: true}` |
| `pulumi import` | No JSON | `{resources: [...], code: "..."}` |
| `pulumi about` | Structured but no `--json` | Current structure wrapped in envelope |

### 2.2 Structured Error Codes and Exit Codes

**Goal**: Define machine-readable error codes and meaningful exit codes.

#### Proposed Error Code System

```go
// Location: pkg/cmd/pulumi/cmd/errors.go (new file)

// Exit codes
const (
    // General errors (0-9)
    ExitCodeSuccess             = 0
    ExitCodeGeneralError        = 1
    ExitCodeInvalidArgs         = 2
    ExitCodeConfigError         = 3

    // Authentication errors (10-19)
    ExitCodeNotLoggedIn         = 10
    ExitCodeAuthExpired         = 11
    ExitCodeInsufficientPerms   = 12

    // Stack errors (20-29)
    ExitCodeStackNotFound       = 20
    ExitCodeStackLocked         = 21
    ExitCodeStackConflict       = 22

    // Operation errors (30-39)
    ExitCodePreviewFailed       = 30
    ExitCodeUpdateFailed        = 31
    ExitCodeDestroyFailed       = 32
    ExitCodeRefreshFailed       = 33
    ExitCodeOperationCancelled  = 34

    // Program errors (40-49)
    ExitCodeCompilationError    = 40
    ExitCodeRuntimeError        = 41
    ExitCodePluginError         = 42

    // State errors (50-59)
    ExitCodeStateCorrupt        = 50
    ExitCodeStateLocked         = 51

    // Policy errors (60-69)
    ExitCodePolicyViolation     = 60
    ExitCodeMandatoryPolicy     = 61
)

// String error codes for JSON output
var ErrorCodes = map[int]string{
    ExitCodeStackNotFound:     "STACK_NOT_FOUND",
    ExitCodeStackLocked:       "STACK_LOCKED",
    ExitCodeNotLoggedIn:       "NOT_LOGGED_IN",
    ExitCodeAuthExpired:       "AUTH_EXPIRED",
    ExitCodeUpdateFailed:      "UPDATE_FAILED",
    ExitCodePolicyViolation:   "POLICY_VIOLATION",
    // ... etc
}
```

#### Error Code Categories

| Range | Category | Examples |
|-------|----------|----------|
| 0-9 | General | Success, invalid args, config error |
| 10-19 | Authentication | Not logged in, expired, insufficient permissions |
| 20-29 | Stack | Not found, locked, conflict |
| 30-39 | Operations | Preview/update/destroy failed, cancelled |
| 40-49 | Program | Compilation, runtime, plugin errors |
| 50-59 | State | Corrupt, locked |
| 60-69 | Policy | Violations, mandatory policy failures |

### 2.3 Agent Mode Flag

**Goal**: Single flag that enables all agent-friendly behaviors.

#### Proposed `--agent` Flag

```go
// Location: pkg/cmd/pulumi/pulumi.go

cmd.PersistentFlags().BoolVar(&agentMode, "agent", false,
    "Enable agent-friendly output mode (implies --json, --non-interactive, structured exit codes)")
```

#### Agent Mode Behavior

When `--agent` is set, the CLI will:

1. **Enable JSON output** - All output uses the standardized JSON envelope
2. **Enable non-interactive mode** - Never prompt for input
3. **Disable cosmetic output** - No emojis, colors, or spinners
4. **Use structured exit codes** - Return specific exit codes instead of generic -1
5. **Separate stdout/stderr** - Data to stdout, diagnostics to stderr
6. **Add operation IDs** - Include unique operation ID in metadata
7. **Use streaming events** - Real-time event streaming for long operations

#### Environment Variable Alternative

```bash
export PULUMI_AGENT_MODE=true
# Equivalent to --agent flag
```

### 2.4 Enhanced CLI Specification

**Goal**: Extend CLI spec to include more metadata for code generation.

#### Proposed Enhancements

```go
// Location: pkg/cmd/pulumi/clispec/gen.go (extend existing)

type Command struct {
    // Existing fields...
    Type          string                   `json:"type"`
    Flags         map[string]Flag          `json:"flags,omitempty"`
    Arguments     *constrictor.Arguments   `json:"arguments,omitempty"`
    Description   string                   `json:"description,omitempty"`

    // New fields for agent usage:
    Examples      []Example                `json:"examples,omitempty"`
    OutputSchema  *JSONSchema              `json:"outputSchema,omitempty"`
    ErrorCodes    []string                 `json:"errorCodes,omitempty"`
    RequiresAuth  bool                     `json:"requiresAuth,omitempty"`
    RequiresStack bool                     `json:"requiresStack,omitempty"`
    Idempotent    bool                     `json:"idempotent,omitempty"`
    MutatesState  bool                     `json:"mutatesState,omitempty"`
}

type Example struct {
    Description string `json:"description"`
    Command     string `json:"command"`
    Output      string `json:"output,omitempty"`
}

type JSONSchema struct {
    // Standard JSON Schema fields
    Type       string                 `json:"type"`
    Properties map[string]interface{} `json:"properties,omitempty"`
    Required   []string               `json:"required,omitempty"`
}
```

#### Benefits for Agents

- **OutputSchema**: Agents can validate and type-check responses
- **ErrorCodes**: Agents know which errors to handle for each command
- **RequiresAuth/RequiresStack**: Agents can check preconditions before running
- **Idempotent**: Agents know which commands are safe to retry
- **Examples**: Provide training data and documentation

### 2.5 Streaming Events for All Operations

**Goal**: Provide real-time structured event streams.

#### Current State

- Preview uses `ShowPreviewDigest()` by default
- Streaming only with `PULUMI_ENABLE_STREAMING_JSON_PREVIEW=true`
- Up/destroy/refresh use streaming when `--json` is passed

#### Proposed Changes

1. Make streaming the default for `--json` mode
2. Add `--json-format` flag for format selection:

```bash
# Streaming events (default with --json)
pulumi up --json --json-format=stream

# Summary digest (backwards compatible)
pulumi up --json --json-format=digest
```

3. Ensure all event types are documented in CLI spec

#### Event Types

```
preludeEvent      - Operation starting
resourcePreEvent  - About to process resource
resourceOutputsEvent - Resource outputs available
resOutputsEvent   - Resource operation complete
diagEvent         - Diagnostic message
policyEvent       - Policy evaluation result
summaryEvent      - Operation complete summary
cancelEvent       - Operation cancelled
```

### 2.6 Idempotent Operations Support

**Goal**: Support safe re-execution of commands.

#### Proposed Flags

```go
// For stack init
cmd.Flags().BoolVar(&ifNotExists, "if-not-exists", false,
    "Only create stack if it doesn't exist (exit 0 if exists)")

// For stack rm
cmd.Flags().BoolVar(&ifExists, "if-exists", false,
    "Only delete stack if it exists (exit 0 if not found)")

// For all operations
cmd.PersistentFlags().StringVar(&operationID, "operation-id", "",
    "Unique operation ID for idempotency (rejects duplicate operations)")
```

#### Idempotent Behavior Examples

```bash
# Create stack if not exists
pulumi stack init dev --if-not-exists
# Returns: exit 0 with result.created=true or result.created=false

# Delete stack if exists
pulumi stack rm dev --if-exists --yes
# Returns: exit 0 with result.deleted=true or result.deleted=false

# Idempotent update (rejects if operation ID already processed)
pulumi up --yes --operation-id="deploy-abc123"
# Returns: exit 0 if already processed, or runs update if new
```

### 2.7 Dry-Run Mode Enhancement

**Goal**: Extend dry-run capabilities beyond preview.

#### Commands to Support `--dry-run`

| Command | Dry-Run Behavior |
|---------|------------------|
| `pulumi config set` | Show what would be set without modifying |
| `pulumi config rm` | Show what would be removed |
| `pulumi state delete` | Show what would be deleted from state |
| `pulumi state rename` | Show the rename that would occur |
| `pulumi import` | Show import plan without modifying state |
| `pulumi stack rm` | Show what would be deleted |

#### Example Output

```bash
pulumi config set myKey myValue --dry-run --json
```

```json
{
  "version": "1.0",
  "command": "config set",
  "success": true,
  "result": {
    "dryRun": true,
    "key": "myKey",
    "newValue": "myValue",
    "oldValue": null,
    "wouldModify": true
  }
}
```

### 2.8 Improved Secrets Handling for Automation

**Goal**: Make secrets handling clearer and more secure for automation.

#### Proposed Changes

```go
// Read secrets from stdin
cmd.Flags().BoolVar(&secretsStdin, "secrets-stdin", false,
    "Read secret values from stdin as JSON")

// Read secrets from file
cmd.Flags().StringVar(&secretsFile, "secrets-file", "",
    "Read secret values from a JSON file")
```

#### Secrets Input Format

```json
{
  "database:password": "secret123",
  "api:key": "sk-abc123"
}
```

#### Usage

```bash
# From stdin
echo '{"database:password": "secret123"}' | pulumi config set-all --secrets-stdin

# From file
pulumi config set-all --secrets-file=/path/to/secrets.json

# Clear error if secret required but not provided
pulumi up --non-interactive
# Error: Secret required for 'database:password'. Use --secrets-stdin or --secrets-file
```

---

## 3. Implementation Priority and Phases

### Phase 1: Foundation (High Priority)

| Item | Description | Files |
|------|-------------|-------|
| 1.1 | Standardized JSON envelope types | `pkg/cmd/pulumi/ui/json_envelope.go` (new) |
| 1.2 | Exit code system | `pkg/cmd/pulumi/cmd/errors.go` (new) |
| 1.3 | Agent mode flag | `pkg/cmd/pulumi/pulumi.go` |
| 1.4 | JSON for `stack init` | `pkg/cmd/pulumi/stack/stack_init.go` |
| 1.5 | JSON for `stack rm` | `pkg/cmd/pulumi/stack/stack_rm.go` |
| 1.6 | JSON for `stack select` | `pkg/cmd/pulumi/stack/stack_select.go` |
| 1.7 | JSON for `login/logout` | `pkg/cmd/pulumi/auth/login.go`, `logout.go` |
| 1.8 | JSON for `new` | `pkg/cmd/pulumi/newcmd/new.go` |

### Phase 2: Enhanced Observability (Medium Priority)

| Item | Description | Files |
|------|-------------|-------|
| 2.1 | Operation IDs | Throughout operation commands |
| 2.2 | Unified streaming events | `pkg/backend/display/json.go` |
| 2.3 | `--json-format` flag | `pkg/cmd/pulumi/operations/*.go` |
| 2.4 | Enhanced CLI spec | `pkg/cmd/pulumi/clispec/gen.go` |
| 2.5 | JSON for `cancel` | `pkg/cmd/pulumi/cancel/cancel.go` |
| 2.6 | JSON for `about` | `pkg/cmd/pulumi/about/about.go` |

### Phase 3: Advanced Features (Lower Priority)

| Item | Description | Files |
|------|-------------|-------|
| 3.1 | `--if-not-exists` for stack init | `pkg/cmd/pulumi/stack/stack_init.go` |
| 3.2 | `--if-exists` for stack rm | `pkg/cmd/pulumi/stack/stack_rm.go` |
| 3.3 | `--operation-id` for idempotency | Backend integration |
| 3.4 | `--dry-run` for config commands | `pkg/cmd/pulumi/config/config.go` |
| 3.5 | `--dry-run` for state commands | `pkg/cmd/pulumi/state/*.go` |
| 3.6 | `--secrets-stdin/file` | `pkg/cmd/pulumi/config/config.go` |
| 3.7 | JSON for `import` | `pkg/cmd/pulumi/convert/import.go` |

---

## 4. Detailed File Changes

### New Files to Create

| File Path | Purpose |
|-----------|---------|
| `pkg/cmd/pulumi/ui/json_envelope.go` | JSON envelope types, helpers, and serialization |
| `pkg/cmd/pulumi/cmd/errors.go` | Error codes, exit codes, and error handling utilities |
| `pkg/cmd/pulumi/cmd/agent.go` | Agent mode implementation and configuration |

### Existing Files to Modify

| File Path | Changes |
|-----------|---------|
| `pkg/cmd/pulumi/pulumi.go` | Add `--agent` flag, wire up agent mode globally |
| `pkg/cmd/pulumi/main.go` | Integrate structured exit codes |
| `pkg/cmd/pulumi/cmd/cmd.go` | Use error code system for error handling |
| `pkg/cmd/pulumi/clispec/gen.go` | Extend CLI spec with new metadata fields |
| `pkg/cmd/pulumi/stack/stack_init.go` | Add `--json`, `--if-not-exists` |
| `pkg/cmd/pulumi/stack/stack_rm.go` | Add `--json`, `--if-exists` |
| `pkg/cmd/pulumi/stack/stack_select.go` | Add `--json` |
| `pkg/cmd/pulumi/newcmd/new.go` | Add `--json`, handle agent mode |
| `pkg/cmd/pulumi/auth/login.go` | Add `--json` |
| `pkg/cmd/pulumi/auth/logout.go` | Add `--json` |
| `pkg/cmd/pulumi/cancel/cancel.go` | Add `--json` |
| `pkg/cmd/pulumi/about/about.go` | Add `--json` |
| `pkg/cmd/pulumi/operations/preview.go` | Unify JSON streaming |
| `pkg/cmd/pulumi/operations/up.go` | Support `--json-format` |
| `pkg/backend/display/json.go` | Support envelope format option |
| `sdk/go/auto/errors.go` | Align with new error codes |

---

## 5. Backwards Compatibility

### JSON Format Changes

- **Version field**: All JSON output includes `"version": "1.0"`
- **Envelope opt-in**: Use `--json-format=envelope` for new format, plain `--json` keeps current behavior initially
- **Deprecation path**:
  1. Release with envelope format as opt-in
  2. Emit deprecation warning for old format
  3. Make envelope default in next major version

### Exit Codes

- **Agent mode only**: Structured exit codes only active with `--agent` flag
- **Default behavior unchanged**: Without `--agent`, maintains current exit code behavior
- **Environment variable**: `PULUMI_AGENT_EXIT_CODES=true` enables structured codes

### Streaming JSON

- **Flag control**: `--json-format=stream|digest` controls format
- **Default unchanged**: Plain `--json` uses current format
- **Agent mode**: `--agent` implies streaming by default

---

## 6. Documentation Requirements

### New Documentation

1. **Agent Integration Guide** (`docs/agent-integration.md`)
   - Overview of agent mode
   - JSON envelope specification
   - Error code reference
   - Best practices for automation

2. **JSON Schema Reference** (`docs/cli-json-schemas.md`)
   - Schema for each command's output
   - Event stream format
   - Error response format

3. **Error Code Reference** (`docs/error-codes.md`)
   - Complete list of error codes
   - Exit code ranges and meanings
   - Troubleshooting guide

### Updated Documentation

1. **CLI Reference** - Add `--agent` flag to all commands
2. **Automation Guide** - Reference new agent mode features
3. **CI/CD Integration** - Examples using agent mode

---

## 7. Testing Strategy

### Unit Tests

- JSON envelope serialization
- Error code mapping
- Agent mode flag propagation
- CLI spec generation with new fields

### Integration Tests

- End-to-end agent mode workflows
- JSON output parsing validation
- Exit code verification for each error type
- Idempotent operation behavior

### Compatibility Tests

- Existing JSON parsers continue working
- Current automation scripts unaffected
- Automation API alignment

---

## 8. Success Metrics

| Metric | Target |
|--------|--------|
| Commands with `--json` support | 100% |
| Commands with structured exit codes | 100% |
| JSON schema coverage in CLI spec | 100% |
| Automation API error code alignment | 100% |
| Documentation coverage | All new features documented |

---

## 9. Open Questions

1. **Envelope format adoption**: Should envelope be opt-in or default?
2. **Operation ID storage**: Where to store operation IDs for idempotency checking?
3. **Streaming format**: NDJSON vs JSON array for event streams?
4. **Schema versioning**: How to handle schema evolution?
5. **Automation API alignment**: Should changes be mirrored to Go/Python/Node SDKs?

---

## 10. Related Work

- **Automation API**: `sdk/go/auto/` - Consider alignment with CLI changes
- **Pulumi Service API**: Ensure CLI agent mode compatible with service
- **Pulumi ESC**: Consider similar improvements for `esc` CLI
- **GitHub CLI**: Reference implementation for agent-friendly CLI design

---

## Appendix A: Example Agent Workflow

```bash
#!/bin/bash
# Example: Automated deployment with full error handling

set -e

# Initialize with agent mode
export PULUMI_AGENT_MODE=true

# Login and capture result
LOGIN_RESULT=$(pulumi login)
if [ "$(echo $LOGIN_RESULT | jq -r '.success')" != "true" ]; then
    echo "Login failed: $(echo $LOGIN_RESULT | jq -r '.error.message')"
    exit $(echo $LOGIN_RESULT | jq -r '.error.code')
fi

# Create stack if not exists
STACK_RESULT=$(pulumi stack init production --if-not-exists)
STACK_CREATED=$(echo $STACK_RESULT | jq -r '.result.created')

# Run preview
PREVIEW_RESULT=$(pulumi preview --json-format=stream)
CHANGES=$(echo "$PREVIEW_RESULT" | jq -s 'map(select(.type=="summaryEvent")) | .[0].changes')

if [ "$CHANGES" = "0" ]; then
    echo "No changes detected"
    exit 0
fi

# Run update with operation ID for idempotency
OPERATION_ID="deploy-$(date +%s)-$(git rev-parse --short HEAD)"
UPDATE_RESULT=$(pulumi up --yes --operation-id="$OPERATION_ID")

if [ "$(echo $UPDATE_RESULT | jq -r '.success')" = "true" ]; then
    echo "Deployment successful"
    echo "Operation ID: $(echo $UPDATE_RESULT | jq -r '.metadata.operationId')"
else
    echo "Deployment failed: $(echo $UPDATE_RESULT | jq -r '.error.message')"
    exit $(echo $UPDATE_RESULT | jq -r '.error.code')
fi
```

---

## Appendix B: JSON Schema Examples

### Stack Init Response

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "version": { "type": "string" },
    "command": { "const": "stack init" },
    "success": { "type": "boolean" },
    "result": {
      "type": "object",
      "properties": {
        "name": { "type": "string" },
        "organization": { "type": "string" },
        "project": { "type": "string" },
        "url": { "type": "string", "format": "uri" },
        "created": { "type": "boolean" }
      },
      "required": ["name", "created"]
    },
    "error": { "$ref": "#/definitions/error" },
    "metadata": { "$ref": "#/definitions/metadata" }
  },
  "required": ["version", "command", "success"]
}
```

### Update Event Stream

```json
{"type": "preludeEvent", "config": {"aws:region": "us-west-2"}}
{"type": "resourcePreEvent", "metadata": {"op": "create", "urn": "urn:pulumi:dev::myapp::aws:s3/bucket:Bucket::mybucket"}}
{"type": "resourceOutputsEvent", "metadata": {"urn": "urn:pulumi:dev::myapp::aws:s3/bucket:Bucket::mybucket"}, "outputs": {"bucket": "mybucket-abc123"}}
{"type": "summaryEvent", "resourceChanges": {"create": 1}, "duration": 5}
```
