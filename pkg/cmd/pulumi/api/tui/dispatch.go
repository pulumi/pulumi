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

package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/api"
)

// dispatchRequest is the payload that the outer model hands to the
// dispatch command. It's constructed from the current Request-tab state
// and carries enough info for a single HTTP round-trip plus optional
// follow-up pagination.
type dispatchRequest struct {
	client      *api.APIClient
	method      string
	path        string
	query       url.Values
	body        []byte
	contentType string
	accept      string
	paginate    bool
	op          *api.Operation
}

// dispatchResultMsg carries the outcome of a single request (or a
// completed multi-page run). For single-shot requests the body is whatever
// came back; for --paginate runs it's the accumulated envelope bytes.
// headers is the last response's header map (paginate: final page).
type dispatchResultMsg struct {
	status  int
	body    []byte
	headers http.Header
	took    time.Duration
	page    int
	items   int // -1 when unknown (non-paginated)
	err     error
}

// dispatchExitMsg is dispatchResultMsg plus a "quit the TUI and emit the
// body to stdout" signal. Using a distinct message type (rather than
// stuffing a sentinel into the error field) keeps the outer Update easy
// to read.
type dispatchExitMsg dispatchResultMsg

// runDispatch returns a tea.Cmd that performs one request, optionally
// following pagination. The returned tea.Msg is a dispatchResultMsg.
func runDispatch(ctx context.Context, req dispatchRequest) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		if !req.paginate {
			resp, err := req.client.Do(ctx, req.method, req.path, req.query,
				bodyReader(req.body), req.contentType, req.accept, nil)
			if err != nil {
				return dispatchResultMsg{err: err, took: time.Since(start)}
			}
			defer resp.Body.Close()
			body, rerr := io.ReadAll(resp.Body)
			if rerr != nil {
				return dispatchResultMsg{err: rerr, took: time.Since(start), status: resp.StatusCode}
			}
			return dispatchResultMsg{
				status:  resp.StatusCode,
				body:    body,
				headers: resp.Header,
				took:    time.Since(start),
				page:    1,
				items:   -1,
			}
		}

		accumulated := make([]byte, 0, 4096)
		var rows int
		var itemsField string
		page := 0
		var cursor, cursorParam string

		lastStatus := 0
		var lastHeaders http.Header
		for page < api.PaginationLimit {
			page++
			q := url.Values{}
			for k, v := range req.query {
				q[k] = append([]string(nil), v...)
			}
			if cursor != "" && cursorParam != "" {
				q.Set(cursorParam, cursor)
			}

			resp, err := req.client.Do(ctx, req.method, req.path, q,
				bodyReader(req.body), req.contentType, req.accept, nil)
			if err != nil {
				return dispatchResultMsg{err: err, took: time.Since(start), status: lastStatus}
			}
			body, rerr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastStatus = resp.StatusCode
			lastHeaders = resp.Header
			if rerr != nil {
				return dispatchResultMsg{err: rerr, took: time.Since(start), status: resp.StatusCode}
			}
			if resp.StatusCode >= 400 {
				return dispatchResultMsg{
					status:  resp.StatusCode,
					body:    body,
					headers: resp.Header,
					took:    time.Since(start),
					page:    page,
					items:   rows,
					err:     fmt.Errorf("HTTP %d on page %d", resp.StatusCode, page),
				}
			}

			items, pageField, nextCursor, nextCursorParam, err := api.ExtractPage(body, req.op)
			if err != nil {
				return dispatchResultMsg{err: err, took: time.Since(start), status: resp.StatusCode}
			}
			if itemsField == "" && pageField != "" {
				itemsField = pageField
			}
			rows += len(items)
			accumulated = appendPageItems(accumulated, items)

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
		}

		final, merr := finalizeAccumulated(accumulated, itemsField)
		if merr != nil {
			return dispatchResultMsg{err: merr, took: time.Since(start), status: lastStatus}
		}

		return dispatchResultMsg{
			status:  lastStatus,
			body:    final,
			headers: lastHeaders,
			took:    time.Since(start),
			page:    page,
			items:   rows,
		}
	}
}

// bodyReader wraps a byte slice in an io.Reader, or returns nil for empty.
func bodyReader(b []byte) io.Reader {
	if len(b) == 0 {
		return nil
	}
	return bytes.NewReader(b)
}

// appendPageItems appends items to an in-memory buffer using the simplest
// valid-JSON-array form: `[e1,e2,...]`. Items are raw JSON. The function
// is intentionally cheap — we only pretty-print once, at finalization.
func appendPageItems(buf []byte, items []json.RawMessage) []byte {
	for _, it := range items {
		if len(buf) > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, it...)
	}
	return buf
}

// finalizeAccumulated wraps the buffer into the final JSON shape that
// downstream viewers (and --jq) will see: bare array when itemsField is
// empty, envelope when it's set.
func finalizeAccumulated(buf []byte, itemsField string) ([]byte, error) {
	arr := append([]byte("["), buf...)
	arr = append(arr, ']')
	if itemsField == "" {
		return arr, nil
	}
	envelope := append([]byte(`{`+strconv.Quote(itemsField)+`:`), arr...)
	envelope = append(envelope, '}')
	return envelope, nil
}
