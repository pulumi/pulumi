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

package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// SchemaVersion is the stable version of every envelope this CLI emits
// (ls, describe, errors, dry-run plan, paginate events).
// Additions are permissive; renames or removals require bumping this constant.
const SchemaVersion = 1

// Exit codes used by `pulumi cloud api` come directly from the shared
// cmdutil taxonomy (sdk/go/common/util/cmdutil/exit.go) so agents get a
// consistent contract across every pulumi subcommand:
//
//   - cmdutil.ExitSuccess (0)             — success
//   - cmdutil.ExitCodeError (1)           — bad flags, no match, template var, partial pagination
//   - cmdutil.ExitConfigurationError (2)  — invalid flag combinations
//   - cmdutil.ExitAuthenticationError (3) — missing auth, 401/403 passthrough
//   - cmdutil.ExitCancelled (8)           — SIGINT / SIGTERM
//   - cmdutil.ExitInternalError (255)     — internal panic / assertion failure
//
// Partial pagination exits 1 and writes the detailed state to a
// `partial_failure` event envelope on stderr.

// Error codes emitted in error envelopes. These form a public contract;
// do not rename. See error_behavior in the plan.
const (
	ErrMissingContext           = "pulumi.cloud_api.missing_context"
	ErrNoMatch                  = "pulumi.cloud_api.no_match"
	ErrInvalidFlags             = "pulumi.cloud_api.invalid_flags"
	ErrRequiredInputMissing     = "pulumi.cloud_api.required_input_missing"
	ErrNotLoggedIn              = "pulumi.cloud_api.not_logged_in"
	ErrNetwork                  = "pulumi.cloud_api.network_error"
	ErrHTTP4xx                  = "pulumi.cloud_api.http_4xx"
	ErrHTTP5xx                  = "pulumi.cloud_api.http_5xx"
	ErrPartialPagination        = "pulumi.cloud_api.partial_pagination"
	ErrUnsupportedSchemaVersion = "pulumi.cloud_api.unsupported_schema_version"
	ErrSpecParse                = "pulumi.cloud_api.spec_parse_error"
	ErrCapabilityNotAvailable   = "pulumi.cloud_api.capability_not_available"
)

// ErrorDetail is the body of an error envelope.
type ErrorDetail struct {
	Code          string   `json:"code"`
	Severity      string   `json:"severity"` // "error" | "warning"
	Message       string   `json:"message"`
	Field         string   `json:"field,omitempty"`
	Suggestions   []string `json:"suggestions,omitempty"`
	SchemaVersion int      `json:"schemaVersion"`
	// HTTPStatus and HTTPBody are populated for ErrHTTP4xx / ErrHTTP5xx.
	HTTPStatus int `json:"httpStatus,omitempty"`
	HTTPBody   any `json:"httpBody,omitempty"`
}

// ErrorEnvelope is the JSON shape emitted on stderr for every CLI-originated error.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// APIError is an error carrying a structured envelope and exit code.
// Wrap/return it from command RunE to emit a parseable error on stderr and
// set a semantic exit code.
type APIError struct {
	ExitCode int
	Envelope ErrorEnvelope
	// Silent suppresses the automatic ErrorEnvelope emission in
	// runWithEnvelope. Use when the command has already written its own
	// structured output (e.g. a partial_failure event during --paginate
	// or a cancellation event from the signal handler).
	Silent bool
}

func (e *APIError) Error() string { return e.Envelope.Error.Message }

// NewAPIError builds an APIError with the given exit code, error code, message,
// and optional extras (suggestions, field, etc.).
func NewAPIError(exit int, code, message string) *APIError {
	return &APIError{
		ExitCode: exit,
		Envelope: ErrorEnvelope{Error: ErrorDetail{
			Code:          code,
			Severity:      "error",
			Message:       message,
			SchemaVersion: SchemaVersion,
		}},
	}
}

// WithField adds a `field` hint to the error envelope.
func (e *APIError) WithField(field string) *APIError {
	e.Envelope.Error.Field = field
	return e
}

// WithSuggestions attaches remediation hints.
func (e *APIError) WithSuggestions(s ...string) *APIError {
	e.Envelope.Error.Suggestions = append(e.Envelope.Error.Suggestions, s...)
	return e
}

// WithHTTP attaches HTTP status + body (for ErrHTTP4xx / ErrHTTP5xx).
func (e *APIError) WithHTTP(status int, body any) *APIError {
	e.Envelope.Error.HTTPStatus = status
	e.Envelope.Error.HTTPBody = body
	return e
}

// WriteJSON emits v as JSON to w using compact when !pretty, indented when pretty.
// Always terminates with a newline so readers splitting on "\n" get clean records.
func WriteJSON(w io.Writer, v any, pretty bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// WriteErrorEnvelope writes err to w in the format that suits the audience.
// When interactive is true the error is rendered as human-readable text:
// `error: <message>` plus optional indented field/HTTPStatus lines and a
// `Suggestions:` block. When interactive is false the structured JSON
// envelope is emitted on a single line so log-line parsers and agents
// can split on "\n" safely. Callers typically pass cmdutil.Interactive().
func WriteErrorEnvelope(w io.Writer, err *APIError, interactive bool) error {
	if interactive {
		return writeErrorText(w, err.Envelope.Error)
	}
	return WriteJSON(w, err.Envelope, false)
}

// writeErrorText renders an ErrorDetail as a human-readable block. Format:
//
//	error: <message>
//	  field: <field>            (when set)
//	  HTTP <status>             (when set)
//
//	Suggestions:                (when any are present)
//	  - <suggestion>
//	  - ...
func writeErrorText(w io.Writer, d ErrorDetail) error {
	var b strings.Builder
	b.WriteString("error: ")
	b.WriteString(d.Message)
	b.WriteByte('\n')
	if d.Field != "" {
		fmt.Fprintf(&b, "  field: %s\n", d.Field)
	}
	if d.HTTPStatus > 0 {
		fmt.Fprintf(&b, "  HTTP %d\n", d.HTTPStatus)
	}
	if len(d.Suggestions) > 0 {
		b.WriteByte('\n')
		b.WriteString("Suggestions:\n")
		for _, s := range d.Suggestions {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// Event envelopes for progress and cancellation. Streamed as JSONL on
// stderr; each line is a complete JSON object.
type Event struct {
	SchemaVersion int    `json:"schemaVersion"`
	Event         string `json:"event"` // e.g. "page", "complete", "partial_failure", "cancelled"
	Timestamp     string `json:"timestamp"`
	// Page-specific fields.
	Page   int `json:"page,omitempty"`
	Count  int `json:"count,omitempty"`
	Cursor any `json:"cursor,omitempty"`
	// Complete-specific fields.
	TotalPages int `json:"totalPages,omitempty"`
	TotalItems int `json:"totalItems,omitempty"`
	// Partial-failure-specific fields.
	PagesFetched int          `json:"pagesFetched,omitempty"`
	FailedAt     int          `json:"failedAt,omitempty"`
	ErrorDetail  *ErrorDetail `json:"error,omitempty"`
	// Cancellation-specific fields.
	Phase      string `json:"phase,omitempty"`
	Completed  []int  `json:"completed,omitempty"`
	InProgress []int  `json:"inProgress,omitempty"`
}

// NewEvent builds an event with schemaVersion and timestamp populated.
func NewEvent(name string, ts time.Time) *Event {
	return &Event{
		SchemaVersion: SchemaVersion,
		Event:         name,
		Timestamp:     ts.UTC().Format(time.RFC3339Nano),
	}
}

// WriteEvent writes a single JSONL event line to w.
func WriteEvent(w io.Writer, ev *Event) error {
	return WriteJSON(w, ev, false)
}

// Command-output envelopes. Each top-level `--format=json` payload gets a
// dedicated envelope type here so the wire format is reviewable in one place.

// orderedByDesc describes the sort order `ls` commits to. Agents can key off
// this so they skip defensive resorting.
const orderedByDesc = "tag asc, path asc, method precedence (GET, POST, PUT, PATCH, DELETE, HEAD)"

// lsOperation is the per-row shape of the `list --format=json` payload.
// Summary and Description both appear: Pulumi's spec generator fills Summary
// from the same Java annotation value as OperationID (so the two often match),
// while Description holds the long-form prose. Both are emitted as "" when
// absent rather than omitted, so jq string predicates like `test(...)` work
// without a null guard.
type lsOperation struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operationId"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Tag         string `json:"tag,omitempty"`
	Preview     bool   `json:"preview,omitempty"`
	Deprecated  bool   `json:"deprecated,omitempty"`
}

// lsEnvelope is the top-level JSON shape emitted by `ls` on stdout when
// piped or when --format=json is set.
type lsEnvelope struct {
	SchemaVersion int           `json:"schemaVersion"`
	OrderedBy     string        `json:"orderedBy"`
	SpecVersion   string        `json:"specVersion,omitempty"`
	Count         int           `json:"count"`
	Operations    []lsOperation `json:"operations"`
}

// describedOp is the per-operation payload emitted by `describe --format=json`.
// It's a view over Operation with stable JSON names so the envelope remains
// usable across CLI versions.
type describedOp struct {
	OperationID     string      `json:"operationId"`
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	Summary         string      `json:"summary"`
	Description     string      `json:"description"`
	Tag             string      `json:"tag,omitempty"`
	Preview         bool        `json:"preview,omitempty"`
	Deprecated      bool        `json:"deprecated,omitempty"`
	SupersededBy    string      `json:"supersededBy,omitempty"`
	Parameters      []ParamSpec `json:"parameters,omitempty"`
	RequestBody     *bodyJSON   `json:"requestBody,omitempty"`
	SuccessResponse *bodyJSON   `json:"successResponse,omitempty"`
}

// bodyJSON is the shape for request / response bodies in the describe
// envelope. `schema` is the human-readable inline rendering; `jsonSchema`
// is the raw OpenAPI schema with all $refs resolved, for agents that want
// to walk the structure programmatically.
type bodyJSON struct {
	ContentType string          `json:"contentType,omitempty"`
	Schema      string          `json:"schema,omitempty"`
	JSONSchema  json.RawMessage `json:"jsonSchema,omitempty"`
}

// describeEnvelope is the top-level `describe --format=json` shape.
type describeEnvelope struct {
	SchemaVersion int         `json:"schemaVersion"`
	Operation     describedOp `json:"operation"`
}

// errorDetailFromErr converts a generic error into a minimal ErrorDetail.
// Intended for wrapping unexpected failures so they still emit structured output.
func errorDetailFromErr(err error) *ErrorDetail {
	return &ErrorDetail{
		Code:          ErrToolError,
		Severity:      "error",
		Message:       fmt.Sprint(err),
		SchemaVersion: SchemaVersion,
	}
}

// ErrToolError is a catch-all code for unexpected internal failures.
const ErrToolError = "pulumi.cloud_api.tool_error"

// runWithEnvelope wraps a RunE implementation so that any returned error is
// emitted as a JSON envelope on stderr and exits with the appropriate
// semantic code. Plain errors are classified as tool errors (exit 255).
//
// The returned error propagates back to main() so the CLI's cleanup path
// (logging flush, OTel span export, profiling close, update notifier) still
// runs before os.Exit. The caller's ExitCodeFor maps *APIError to the right
// semantic exit code, and Silent is set after the envelope is written so the
// generic DisplayErrorMessage path does not re-print the message.
func runWithEnvelope(fn func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := fn(cmd, args)
		if err == nil {
			return nil
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			apiErr = &APIError{
				ExitCode: cmdutil.ExitInternalError,
				Envelope: ErrorEnvelope{Error: *errorDetailFromErr(err)},
			}
		}
		if !apiErr.Silent {
			_ = WriteErrorEnvelope(cmd.ErrOrStderr(), apiErr, cmdutil.Interactive())
			apiErr.Silent = true
		}
		return apiErr
	}
}
