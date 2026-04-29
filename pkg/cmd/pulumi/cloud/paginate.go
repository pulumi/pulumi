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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// PaginationLimit caps the number of pages we'll follow. Higher than any
// realistic list endpoint; keeps a runaway loop from hanging the CLI if the
// service returns a cursor that never terminates. Declared as var so tests
// can lower it without having to fake 1000+ network round trips.
var PaginationLimit = 1000

// paginateRequest is the subset of an outgoing request that the paginate
// loop needs to mutate the cursor on each iteration.
type paginateRequest struct {
	Method, Path string
	BaseQuery    url.Values
	Body         []byte
	ContentType  string
	Accept       string
	Headers      []ParsedHeader
	// Op is consulted when the response cursor field is named differently from the
	// request parameter (e.g. "nextToken" in response, "continuationToken" in query).
	Op *Operation
}

// runPaginate executes req repeatedly, following continuationToken cursors,
// and concatenates items into a single JSON document on w.
//
// Returns a Silent APIError on partial failure so the caller's wrapper
// doesn't double-emit — the accumulated pages already on w are the ground
// truth.
func runPaginate(
	ctx context.Context,
	w io.Writer,
	apiClient *client.Client,
	req paginateRequest,
	flags *apiCommand,
) error {
	accumulated := make([]json.RawMessage, 0, 256)
	page := 0
	var cursor, cursorParam string
	// itemsField is locked in from the first page that has one, so the accumulated
	// output can be wrapped in the same envelope shape a single-page response would
	// produce — preserving downstream `| jq` compatibility. Empty means the server
	// returned a bare array.
	var itemsField string
	// truncated is set only when the loop exits because it hit PaginationLimit
	// with a live cursor; every other exit path (natural end, safety valve)
	// leaves it false.
	var truncated bool

	for page < PaginationLimit {
		page++

		if errors.Is(ctx.Err(), context.Canceled) {
			return emitCancelled(w, flags, accumulated, itemsField)
		}

		q := url.Values{}
		for k, v := range req.BaseQuery {
			q[k] = append([]string(nil), v...)
		}
		if cursor != "" && cursorParam != "" {
			q.Set(cursorParam, cursor)
		}

		var bodyReader io.Reader
		if len(req.Body) > 0 {
			bodyReader = bytes.NewReader(req.Body)
		}
		resp, err := apiClient.RawCall(ctx, req.Method, req.Path, q, bodyReader,
			buildAPIHeaders(req.ContentType, req.Accept, req.Headers), len(req.Body) > 0)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return emitCancelled(w, flags, accumulated, itemsField)
			}
			return emitPartialFailure(w, flags, accumulated, itemsField,
				NewAPIError(cmdutil.ExitCodeError, ErrNetwork,
					fmt.Sprintf("HTTP %s %s: %v", req.Method, req.Path, err)))
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return emitPartialFailure(w, flags, accumulated, itemsField,
				NewAPIError(cmdutil.ExitCodeError, ErrNetwork,
					fmt.Sprintf("reading page %d: %v", page, err)))
		}

		if resp.StatusCode >= 400 {
			return emitPartialFailure(w, flags, accumulated, itemsField, httpErrorEnvelopeBytes(resp, body))
		}

		items, pageItemsField, nextCursor, nextCursorParam, err := ExtractPage(body, req.Op)
		if err != nil {
			return emitPartialFailure(w, flags, accumulated, itemsField,
				NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
					fmt.Sprintf("page %d: %v; --paginate requires a JSON shape with an array and "+
						"optional continuationToken/cursor/nextToken", page, err)))
		}
		if itemsField == "" && pageItemsField != "" {
			itemsField = pageItemsField
		}

		accumulated = append(accumulated, items...)

		// No-progress safety valve: if the server hands us back the same
		// cursor we just sent, or returns a non-empty cursor alongside an
		// empty page, we're in a loop. Stop rather than spin.
		if nextCursor != "" && nextCursor == cursor {
			break
		}
		if nextCursor != "" && len(items) == 0 {
			break
		}

		cursor = nextCursor
		cursorParam = nextCursorParam
		if cursor == "" {
			break
		}
		if page == PaginationLimit {
			truncated = true
		}
	}

	// Loop exited via the `page < PaginationLimit` guard with a live cursor:
	// the dataset was truncated, not exhausted. Emit a partial-pagination
	// error so agents can detect it via exit code + code field, and resume
	// from the returned cursor.
	if truncated {
		flushAccumulated(w, accumulated, itemsField, flags)
		apiErr := NewAPIError(cmdutil.ExitCodeError, ErrPartialPagination,
			fmt.Sprintf("pagination truncated at %d pages; %d items collected; more data available",
				PaginationLimit, len(accumulated))).
			WithSuggestions("re-run with the returned cursor to resume")
		apiErr.Silent = true
		return apiErr
	}

	return writeAccumulated(w, accumulated, itemsField, flags)
}

// ExtractPage parses a JSON page and returns its items, the name of the
// envelope field those items came from (empty when the page was a bare
// array), the next cursor, and the query-parameter name to send that
// cursor as on the next request.
//
// Supported shapes:
//   - Top-level JSON array: items; no cursor.
//   - Object with top-level "continuationToken" / "cursor" string: cursor param matches the field name.
//   - Object with top-level "nextToken": cursor param resolved from op.Params (errors if unknown).
//   - Object with nested "pagination": { "next": "<url>" } — v2; cursor param is parsed from the URL.
//
// In all object shapes the items collection is the sole array-valued top-level field.
func ExtractPage(body []byte, op *Operation) ([]json.RawMessage, string, string, string, error) {
	if len(body) == 0 {
		return nil, "", "", "", nil
	}
	// Try array first.
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, "", "", "", nil
	}
	// Else expect object.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, "", "", "", fmt.Errorf("response is neither a JSON array nor object: %w", err)
	}

	cursor, cursorParam, err := extractCursor(obj, op)
	if err != nil {
		return nil, "", "", "", err
	}

	// Find the sole array-valued field at the top level.
	var itemsField string
	var items []json.RawMessage
	for key, raw := range obj {
		if key == "continuationToken" || key == "cursor" || key == "nextToken" {
			continue
		}
		trim := bytes.TrimSpace(raw)
		if len(trim) > 0 && trim[0] == '[' {
			if itemsField != "" {
				return nil, "", "", "", fmt.Errorf("ambiguous paginated shape: multiple array fields (%q, %q)",
					itemsField, key)
			}
			var a []json.RawMessage
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, "", "", "", fmt.Errorf("decoding field %q: %w", key, err)
			}
			itemsField = key
			items = a
		}
	}
	if itemsField == "" {
		// No array field + no cursor → legitimate empty page (some endpoints return `{}`
		// instead of an empty array envelope). Cursor + no array would loop forever.
		if cursor == "" {
			return nil, "", "", "", nil
		}
		return nil, "", cursor, cursorParam,
			fmt.Errorf("response has a %q cursor but no array field", cursorParam)
	}
	return items, itemsField, cursor, cursorParam, nil
}

// cursorParamCandidates is the ordered list of query-parameter names
// recognised as "the pagination cursor" when a response uses a cursor
// field whose name doesn't match the request parameter (e.g. "nextToken"
// in the response paired with "continuationToken" in the query). Order
// is preference — more-specific first.
var cursorParamCandidates = []string{"continuationToken", "cursor", "nextToken", "pageToken"}

// pickCursorParam returns the first query parameter on op whose name is
// listed in cursorParamCandidates. Returns "" when op is nil or exposes
// no recognised pagination parameter.
func pickCursorParam(op *Operation) string {
	if op == nil {
		return ""
	}
	for _, name := range cursorParamCandidates {
		for _, p := range op.Params {
			if p.In == "query" && p.Name == name {
				return name
			}
		}
	}
	return ""
}

// extractCursor looks for a continuation cursor in a response object and
// returns the cursor value and the query-parameter name the next request
// should send it under. Returns ("", "", nil) if no cursor is present
// (= no more pages).
//
// For the v2 pagination shape ({"pagination": {"cursor": ..., "next":
// ..., "previous": ...}}) only "next" indicates there is a further page:
// the schema documents "cursor" as "an opaque cursor for resuming
// pagination" (a bookmark of the *current* position, which the server
// returns on every page), while "next" is "Link to the next page of
// results" — an empty "next" means the last page. We therefore extract
// the cursor from the "next" URL's query string.
//
// For "nextToken", the query-parameter name does not match the response field, so we
// consult op.Params via pickCursorParam.
func extractCursor(obj map[string]json.RawMessage, op *Operation) (cursor, param string, err error) {
	if raw, ok := obj["continuationToken"]; ok {
		var s string
		if jerr := json.Unmarshal(raw, &s); jerr == nil && s != "" {
			return s, "continuationToken", nil
		}
	}
	if raw, ok := obj["cursor"]; ok {
		var s string
		if jerr := json.Unmarshal(raw, &s); jerr == nil && s != "" {
			return s, "cursor", nil
		}
	}
	if raw, ok := obj["nextToken"]; ok {
		var s string
		if jerr := json.Unmarshal(raw, &s); jerr == nil && s != "" {
			name := pickCursorParam(op)
			if name == "" {
				return "", "", errors.New(
					"response has nextToken but operation exposes no recognised pagination query parameter " +
						"(expected one of continuationToken, cursor, nextToken, pageToken)")
			}
			return s, name, nil
		}
	}
	if raw, ok := obj["pagination"]; ok {
		var pag map[string]json.RawMessage
		if jerr := json.Unmarshal(raw, &pag); jerr == nil {
			if n, ok := pag["next"]; ok {
				var link string
				if jerr := json.Unmarshal(n, &link); jerr == nil && link != "" {
					if c, p := cursorFromLink(link); c != "" {
						return c, p, nil
					}
				}
			}
		}
	}
	return "", "", nil
}

// cursorFromLink extracts the advance marker from a pagination-next link
// (either a full URL or a bare path+query). Returns the value and the
// query-param name to send it as. Recognises both cursor-style
// ("cursor"/"continuationToken") and offset-style ("page"/"offset")
// advance markers. Returns ("", "") when no recognised marker is
// present.
func cursorFromLink(link string) (cursor, param string) {
	u, err := url.Parse(link)
	if err != nil {
		return "", ""
	}
	q := u.Query()
	for _, name := range []string{"cursor", "continuationToken", "page", "offset"} {
		if v := q.Get(name); v != "" {
			return v, name
		}
	}
	return "", ""
}

// writeAccumulated writes the combined result to w. When itemsField is
// non-empty (the server returned an object envelope with a named array), the
// output is wrapped back into that same shape ({<itemsField>: [...]}) so
// downstream `| jq` filters work identically whether or not --paginate is set.
// When itemsField is empty (server returned a bare array), the output stays
// a bare array.
func writeAccumulated(w io.Writer, items []json.RawMessage, itemsField string, flags *apiCommand) error {
	combined, err := marshalAccumulated(items, itemsField)
	if err != nil {
		return NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("serializing paginated results: %v", err))
	}
	if flags.silent {
		return nil
	}
	if cmdutil.Interactive() {
		var buf bytes.Buffer
		if err := json.Indent(&buf, combined, "", "  "); err == nil {
			combined = buf.Bytes()
		}
	}
	if _, err := w.Write(combined); err != nil {
		return err
	}
	if len(combined) > 0 && combined[len(combined)-1] != '\n' {
		fmt.Fprintln(w)
	}
	return nil
}

// marshalAccumulated serializes the accumulated items, wrapping them in
// {<itemsField>: [...]} when itemsField is set. Cursor fields are
// intentionally omitted from the wrapped envelope — they are meaningless
// after pagination completes.
func marshalAccumulated(items []json.RawMessage, itemsField string) ([]byte, error) {
	if itemsField == "" {
		return json.Marshal(items)
	}
	return json.Marshal(map[string]any{itemsField: items})
}

// Best-effort flush preserves the envelope shape so downstream consumers don't
// see a different type on failure. Honors --silent (no body output), matching
// the behaviour of the success-path writeAccumulated.
func flushAccumulated(w io.Writer, accumulated []json.RawMessage, itemsField string, flags *apiCommand) {
	if flags != nil && flags.silent {
		return
	}
	combined, err := marshalAccumulated(accumulated, itemsField)
	if err != nil {
		return
	}
	_, _ = w.Write(combined)
	fmt.Fprintln(w)
}

// emitPartialFailure flushes accumulated pages to w and returns the
// underlying APIError marked Silent so the envelope contains the failed
// page's error detail rather than the per-page printed body.
func emitPartialFailure(
	w io.Writer, flags *apiCommand,
	accumulated []json.RawMessage, itemsField string, apiErr *APIError,
) error {
	flushAccumulated(w, accumulated, itemsField, flags)
	apiErr.Silent = true
	return apiErr
}

// emitCancelled flushes accumulated pages to w and returns a Silent APIError
// with cmdutil.ExitCancelled.
func emitCancelled(
	w io.Writer, flags *apiCommand,
	accumulated []json.RawMessage, itemsField string,
) error {
	flushAccumulated(w, accumulated, itemsField, flags)
	return &APIError{ExitCode: cmdutil.ExitCancelled, Silent: true}
}
